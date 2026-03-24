package backend

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"text/template"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/codebuild"
	cbtypes "github.com/aws/aws-sdk-go-v2/service/codebuild/types"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

// BuildRequest defines the parameters for a remote container build.
type BuildRequest struct {
	ProjectName   string
	ContextBucket string
	ContextKey    string
	Dockerfile    string
	Target        string
	Platforms     []string
	Tags          []string
	BuildArgs     map[string]string
	Labels        map[string]string
	ResultBucket  string
	ResultKey     string
}

// BuildResult contains the outputs from a completed build.
type BuildResult struct {
	BuildID   string
	Status    string
	StartTime string
	EndTime   string
	LogURL    string
	Digest    string
	ImageRef  string
}

// ResultMetadata is the JSON structure written by the buildspec to S3.
type ResultMetadata struct {
	Digest   string `json:"digest"`
	ImageRef string `json:"image_ref"`
}

// GetProjectS3Config reads a CodeBuild project and extracts the S3 bucket and
// prefix from its source configuration.
func GetProjectS3Config(ctx context.Context, client *codebuild.Client, projectName string) (bucket, prefix string, err error) {
	output, err := client.BatchGetProjects(ctx, &codebuild.BatchGetProjectsInput{
		Names: []string{projectName},
	})
	if err != nil {
		return "", "", fmt.Errorf("describing project %q: %w", projectName, err)
	}
	if len(output.Projects) == 0 {
		return "", "", fmt.Errorf("project %q not found", projectName)
	}

	project := output.Projects[0]
	if project.Source == nil || project.Source.Location == nil {
		return "", "", fmt.Errorf("project %q has no S3 source configured", projectName)
	}

	location := aws.ToString(project.Source.Location)
	parts := strings.SplitN(location, "/", 2)
	if len(parts) < 2 {
		return location, "", nil
	}
	return parts[0], parts[1], nil
}

// UploadContext uploads a local archive file to S3.
func UploadContext(ctx context.Context, client *s3.Client, bucket, key, archivePath string) error {
	f, err := os.Open(archivePath)
	if err != nil {
		return fmt.Errorf("opening archive %q: %w", archivePath, err)
	}
	defer f.Close()

	_, err = client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
		Body:   f,
	})
	if err != nil {
		return fmt.Errorf("uploading to s3://%s/%s: %w", bucket, key, err)
	}

	return nil
}

// StartBuild initiates a CodeBuild build with the given parameters.
func StartBuild(ctx context.Context, client *codebuild.Client, req *BuildRequest) (string, error) {
	buildspec := GenerateBuildspec(req)

	envVars := []cbtypes.EnvironmentVariable{
		{Name: aws.String("WHARFLAB_DOCKERFILE"), Value: aws.String(req.Dockerfile), Type: cbtypes.EnvironmentVariableTypePlaintext},
		{Name: aws.String("WHARFLAB_PLATFORMS"), Value: aws.String(strings.Join(req.Platforms, ",")), Type: cbtypes.EnvironmentVariableTypePlaintext},
		{Name: aws.String("WHARFLAB_TAGS"), Value: aws.String(strings.Join(req.Tags, " ")), Type: cbtypes.EnvironmentVariableTypePlaintext},
		{Name: aws.String("WHARFLAB_RESULT_BUCKET"), Value: aws.String(req.ResultBucket), Type: cbtypes.EnvironmentVariableTypePlaintext},
		{Name: aws.String("WHARFLAB_RESULT_KEY"), Value: aws.String(req.ResultKey), Type: cbtypes.EnvironmentVariableTypePlaintext},
	}

	if req.Target != "" {
		envVars = append(envVars, cbtypes.EnvironmentVariable{
			Name: aws.String("WHARFLAB_TARGET"), Value: aws.String(req.Target),
			Type: cbtypes.EnvironmentVariableTypePlaintext,
		})
	}

	if len(req.BuildArgs) > 0 {
		argsJSON, _ := json.Marshal(req.BuildArgs)
		envVars = append(envVars, cbtypes.EnvironmentVariable{
			Name: aws.String("WHARFLAB_BUILD_ARGS"), Value: aws.String(string(argsJSON)),
			Type: cbtypes.EnvironmentVariableTypePlaintext,
		})
	}

	if len(req.Labels) > 0 {
		labelsJSON, _ := json.Marshal(req.Labels)
		envVars = append(envVars, cbtypes.EnvironmentVariable{
			Name: aws.String("WHARFLAB_LABELS"), Value: aws.String(string(labelsJSON)),
			Type: cbtypes.EnvironmentVariableTypePlaintext,
		})
	}

	sourceLocation := fmt.Sprintf("%s/%s", req.ContextBucket, req.ContextKey)

	input := &codebuild.StartBuildInput{
		ProjectName:                  aws.String(req.ProjectName),
		SourceTypeOverride:           cbtypes.SourceTypeS3,
		SourceLocationOverride:       aws.String(sourceLocation),
		BuildspecOverride:            aws.String(buildspec),
		EnvironmentVariablesOverride: envVars,
	}

	output, err := client.StartBuild(ctx, input)
	if err != nil {
		return "", fmt.Errorf("starting build: %w", err)
	}

	return aws.ToString(output.Build.Id), nil
}

// WaitForBuild polls until the build completes, returning the result.
func WaitForBuild(ctx context.Context, client *codebuild.Client, buildID string) (*BuildResult, error) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-ticker.C:
			tflog.Debug(ctx, "polling build status", map[string]interface{}{"build_id": buildID})

			output, err := client.BatchGetBuilds(ctx, &codebuild.BatchGetBuildsInput{
				Ids: []string{buildID},
			})
			if err != nil {
				return nil, fmt.Errorf("polling build %s: %w", buildID, err)
			}
			if len(output.Builds) == 0 {
				return nil, fmt.Errorf("build %s not found", buildID)
			}

			build := output.Builds[0]
			result := &BuildResult{
				BuildID: buildID,
				Status:  string(build.BuildStatus),
			}
			if build.StartTime != nil {
				result.StartTime = build.StartTime.Format(time.RFC3339)
			}
			if build.EndTime != nil {
				result.EndTime = build.EndTime.Format(time.RFC3339)
			}
			if build.Logs != nil && build.Logs.DeepLink != nil {
				result.LogURL = aws.ToString(build.Logs.DeepLink)
			}

			switch build.BuildStatus {
			case cbtypes.StatusTypeSucceeded:
				return result, nil
			case cbtypes.StatusTypeFailed:
				msg := "build failed"
				if phases := build.Phases; len(phases) > 0 {
					last := phases[len(phases)-1]
					if last.PhaseStatus != "" {
						msg = fmt.Sprintf("build failed at phase %s: %s", last.PhaseType, last.PhaseStatus)
					}
					for _, pc := range last.Contexts {
						if pc.Message != nil {
							msg += ": " + aws.ToString(pc.Message)
						}
					}
				}
				return result, fmt.Errorf("%s (build ID: %s, log: %s)", msg, buildID, result.LogURL)
			case cbtypes.StatusTypeStopped:
				return result, fmt.Errorf("build was stopped (build ID: %s)", buildID)
			case cbtypes.StatusTypeTimedOut:
				return result, fmt.Errorf("build timed out (build ID: %s)", buildID)
			case cbtypes.StatusTypeFault:
				return result, fmt.Errorf("build fault: internal error (build ID: %s)", buildID)
			}
			// StatusTypeInProgress — keep polling.
		}
	}
}

// ReadResultMetadata reads the build result JSON from S3.
func ReadResultMetadata(ctx context.Context, client *s3.Client, bucket, key string) (*ResultMetadata, error) {
	output, err := client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, fmt.Errorf("reading result from s3://%s/%s: %w", bucket, key, err)
	}
	defer output.Body.Close()

	body, err := io.ReadAll(output.Body)
	if err != nil {
		return nil, fmt.Errorf("reading result body: %w", err)
	}

	var meta ResultMetadata
	if err := json.Unmarshal(body, &meta); err != nil {
		return nil, fmt.Errorf("parsing result JSON: %w (body: %s)", err, string(body))
	}

	return &meta, nil
}

// CleanupResult deletes the build result metadata from S3.
func CleanupResult(ctx context.Context, client *s3.Client, bucket, key string) error {
	_, err := client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	return err
}

// CleanupContext deletes the uploaded context archive from S3.
func CleanupContext(ctx context.Context, client *s3.Client, bucket, key string) error {
	_, err := client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	return err
}

var buildspecTemplate = template.Must(template.New("buildspec").Parse(`version: 0.2
phases:
  pre_build:
    commands:
      - echo "Logging into container registries..."
      - |
        for tag in $WHARFLAB_TAGS; do
          registry="${tag%%/*}"
          if echo "$registry" | grep -q "dkr.ecr.*\.amazonaws\.com"; then
            region=$(echo "$registry" | sed -n 's/.*\.ecr\.\([^.]*\)\.amazonaws\.com/\1/p')
            echo "Logging into ECR in $region..."
            aws ecr get-login-password --region "$region" | docker login --username AWS --password-stdin "$registry" 2>/dev/null
          fi
        done
  build:
    commands:
      - echo "Starting container build..."
      - |
        TAG_FLAGS=""
        for tag in $WHARFLAB_TAGS; do
          TAG_FLAGS="$TAG_FLAGS -t $tag"
        done

        BUILD_ARG_FLAGS=""
        if [ -n "$WHARFLAB_BUILD_ARGS" ]; then
          for key in $(echo "$WHARFLAB_BUILD_ARGS" | jq -r 'keys[]' 2>/dev/null); do
            val=$(echo "$WHARFLAB_BUILD_ARGS" | jq -r --arg k "$key" '.[$k]')
            BUILD_ARG_FLAGS="$BUILD_ARG_FLAGS --build-arg $key=$val"
          done
        fi

        LABEL_FLAGS=""
        if [ -n "$WHARFLAB_LABELS" ]; then
          for key in $(echo "$WHARFLAB_LABELS" | jq -r 'keys[]' 2>/dev/null); do
            val=$(echo "$WHARFLAB_LABELS" | jq -r --arg k "$key" '.[$k]')
            LABEL_FLAGS="$LABEL_FLAGS --label $key=$val"
          done
        fi

        TARGET_FLAG=""
        if [ -n "$WHARFLAB_TARGET" ]; then
          TARGET_FLAG="--target $WHARFLAB_TARGET"
        fi

        docker buildx build \
          -f "$WHARFLAB_DOCKERFILE" \
          --platform "$WHARFLAB_PLATFORMS" \
          $TAG_FLAGS \
          $BUILD_ARG_FLAGS \
          $LABEL_FLAGS \
          $TARGET_FLAG \
          --push \
          --provenance=false \
          --metadata-file /tmp/build-metadata.json \
          .
  post_build:
    commands:
      - echo "Capturing build metadata..."
      - |
        DIGEST=$(jq -r '."containerimage.digest" // empty' /tmp/build-metadata.json 2>/dev/null)
        if [ -z "$DIGEST" ]; then
          DIGEST=$(jq -r '.digest // empty' /tmp/build-metadata.json 2>/dev/null)
        fi
        FIRST_TAG=$(echo "$WHARFLAB_TAGS" | awk '{print $1}')
        IMAGE_REF="${FIRST_TAG}@${DIGEST}"
        printf '{"digest":"%s","image_ref":"%s"}\n' "$DIGEST" "$IMAGE_REF" > /tmp/wharflab-result.json
        aws s3 cp /tmp/wharflab-result.json "s3://${WHARFLAB_RESULT_BUCKET}/${WHARFLAB_RESULT_KEY}"
        echo "Build complete. Digest: $DIGEST"
`))

// GenerateBuildspec produces the CodeBuild buildspec YAML for a container build.
func GenerateBuildspec(req *BuildRequest) string {
	var buf bytes.Buffer
	_ = buildspecTemplate.Execute(&buf, req)
	return buf.String()
}
