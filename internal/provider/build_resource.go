package provider

import (
	"context"
	"crypto/rand"
	"fmt"
	"os"
	"strings"

	"github.com/aws/aws-sdk-go-v2/service/codebuild"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"github.com/wharflab/terraform-provider-containers/internal/backend"
	"github.com/wharflab/terraform-provider-containers/internal/buildcontext"
)

var (
	_ resource.Resource                   = &buildResource{}
	_ resource.ResourceWithConfigure      = &buildResource{}
	_ resource.ResourceWithModifyPlan     = &buildResource{}
)

type buildResource struct {
	providerData *ProviderData
}

// --- Models ---

type buildContextModel struct {
	Type   types.String `tfsdk:"type"`
	Path   types.String `tfsdk:"path"`
	URL    types.String `tfsdk:"url"`
	Ref    types.String `tfsdk:"ref"`
	Bucket types.String `tfsdk:"bucket"`
	Key    types.String `tfsdk:"key"`
}

type buildDefinitionModel struct {
	Dockerfile types.String `tfsdk:"dockerfile"`
	Target     types.String `tfsdk:"target"`
}

type buildResourceModel struct {
	ID               types.String          `tfsdk:"id"`
	BuilderID        types.String          `tfsdk:"builder_id"`
	Context          *buildContextModel    `tfsdk:"context"`
	Definition       *buildDefinitionModel `tfsdk:"definition"`
	Platforms        types.List            `tfsdk:"platforms"`
	Tags             types.List            `tfsdk:"tags"`
	BuildArgs        types.Map             `tfsdk:"build_args"`
	Labels           types.Map             `tfsdk:"labels"`
	Push             types.Bool            `tfsdk:"push"`
	// Computed outputs
	ImageDigest      types.String `tfsdk:"image_digest"`
	ImageRef         types.String `tfsdk:"image_ref"`
	ImageIndexDigest types.String `tfsdk:"image_index_digest"`
	ContextHash      types.String `tfsdk:"context_hash"`
	BuildID          types.String `tfsdk:"build_id"`
	BuildStatus      types.String `tfsdk:"build_status"`
	BuildStartTime   types.String `tfsdk:"build_start_time"`
	BuildEndTime     types.String `tfsdk:"build_end_time"`
	LogURL           types.String `tfsdk:"log_url"`
}

// --- Constructor ---

func NewBuildResource() resource.Resource {
	return &buildResource{}
}

// --- Metadata ---

func (r *buildResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_build"
}

// --- Schema ---

func (r *buildResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Executes a remote container build and publishes OCI images.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "Unique identifier for this build resource.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"builder_id": schema.StringAttribute{
				Required:    true,
				Description: "ID of the builder resource (e.g. containers_aws_codebuild_builder).",
			},
			"platforms": schema.ListAttribute{
				Optional:    true,
				Computed:    true,
				ElementType: types.StringType,
				Description: "Target platforms. Defaults to [\"linux/amd64\"].",
			},
			"tags": schema.ListAttribute{
				Required:    true,
				ElementType: types.StringType,
				Description: "Image tags/references to publish (e.g. '123456789012.dkr.ecr.eu-west-1.amazonaws.com/app:latest').",
			},
			"build_args": schema.MapAttribute{
				Optional:    true,
				ElementType: types.StringType,
				Description: "Docker build arguments.",
			},
			"labels": schema.MapAttribute{
				Optional:    true,
				ElementType: types.StringType,
				Description: "Image labels.",
			},
			"push": schema.BoolAttribute{
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(true),
				Description: "Push built image to registry. Defaults to true.",
			},
			// Computed outputs
			"image_digest": schema.StringAttribute{
				Computed:    true,
				Description: "Image digest (e.g. sha256:abc123...).",
			},
			"image_ref": schema.StringAttribute{
				Computed:    true,
				Description: "Digest-qualified image reference.",
			},
			"image_index_digest": schema.StringAttribute{
				Computed:    true,
				Description: "Image index digest for multi-platform builds.",
			},
			"context_hash": schema.StringAttribute{
				Computed:    true,
				Description: "SHA256 hash of the build context. Changes trigger a rebuild.",
			},
			"build_id": schema.StringAttribute{
				Computed:    true,
				Description: "Backend build execution ID.",
			},
			"build_status": schema.StringAttribute{
				Computed:    true,
				Description: "Final build status.",
			},
			"build_start_time": schema.StringAttribute{
				Computed:    true,
				Description: "Build start time (RFC 3339).",
			},
			"build_end_time": schema.StringAttribute{
				Computed:    true,
				Description: "Build end time (RFC 3339).",
			},
			"log_url": schema.StringAttribute{
				Computed:    true,
				Description: "URL to the build log.",
			},
		},
		Blocks: map[string]schema.Block{
			"context": schema.SingleNestedBlock{
				Description: "Build context source. Required.",
				Attributes: map[string]schema.Attribute{
					"type": schema.StringAttribute{
						Required:    true,
						Description: "Context source type: 'local_directory', 'git', 's3', or 'tarball'.",
					},
					"path": schema.StringAttribute{
						Optional:    true,
						Description: "Local directory path (for type 'local_directory').",
					},
					"url": schema.StringAttribute{
						Optional:    true,
						Description: "Git repository URL (for type 'git').",
					},
					"ref": schema.StringAttribute{
						Optional:    true,
						Description: "Git ref/branch/tag (for type 'git').",
					},
					"bucket": schema.StringAttribute{
						Optional:    true,
						Description: "S3 bucket (for type 's3').",
					},
					"key": schema.StringAttribute{
						Optional:    true,
						Description: "S3 object key (for type 's3').",
					},
				},
			},
			"definition": schema.SingleNestedBlock{
				Description: "Build definition. Defaults to Dockerfile in context root.",
				Attributes: map[string]schema.Attribute{
					"dockerfile": schema.StringAttribute{
						Optional:    true,
						Computed:    true,
						Default:     stringdefault.StaticString("Dockerfile"),
						Description: "Path to the Dockerfile relative to the context root. Defaults to 'Dockerfile'.",
					},
					"target": schema.StringAttribute{
						Optional:    true,
						Description: "Target build stage name.",
					},
				},
			},
		},
	}
}

// --- Configure ---

func (r *buildResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	data, ok := req.ProviderData.(*ProviderData)
	if !ok {
		resp.Diagnostics.AddError("Unexpected Provider Data", fmt.Sprintf("Expected *ProviderData, got %T", req.ProviderData))
		return
	}
	r.providerData = data
}

// --- Plan modification: compute context_hash during planning ---

func (r *buildResource) ModifyPlan(ctx context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) {
	// Nothing to do on destroy.
	if req.Plan.Raw.IsNull() {
		return
	}

	var plan buildResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Set default platforms if not specified.
	if plan.Platforms.IsNull() || plan.Platforms.IsUnknown() {
		plan.Platforms, resp.Diagnostics = types.ListValueFrom(ctx, types.StringType, []string{"linux/amd64"})
		if resp.Diagnostics.HasError() {
			return
		}
	}

	// Compute context hash for local_directory contexts.
	if plan.Context != nil &&
		!plan.Context.Type.IsUnknown() &&
		plan.Context.Type.ValueString() == string(buildcontext.ContextTypeLocal) &&
		!plan.Context.Path.IsUnknown() {

		hash, err := buildcontext.HashLocalContext(plan.Context.Path.ValueString())
		if err != nil {
			resp.Diagnostics.AddWarning(
				"Cannot compute context hash",
				fmt.Sprintf("Could not hash directory %q during plan: %s. Build will proceed during apply.", plan.Context.Path.ValueString(), err),
			)
		} else {
			plan.ContextHash = types.StringValue(hash)
		}
	}

	resp.Diagnostics.Append(resp.Plan.Set(ctx, &plan)...)
}

// --- CRUD ---

func (r *buildResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan buildResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if plan.Context == nil {
		resp.Diagnostics.AddError("Missing required block", "The 'context' block is required.")
		return
	}

	plan.ID = types.StringValue(generateID())
	r.executeBuild(ctx, &plan, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *buildResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state buildResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	// MVP: return stored state without active registry verification.
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *buildResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan buildResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Preserve the resource ID across updates.
	var state buildResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	plan.ID = state.ID

	r.executeBuild(ctx, &plan, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *buildResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	// Build resources don't own the published image. Just remove from state.
	// Future: clean up S3 context/metadata artifacts.
	tflog.Debug(ctx, "build resource deleted from state")
}

// --- Core build execution ---

func (r *buildResource) executeBuild(ctx context.Context, plan *buildResourceModel, diags *diag.Diagnostics) {
	cfg := r.providerData.AWSConfig
	cbClient := codebuild.NewFromConfig(cfg)
	s3Client := s3.NewFromConfig(cfg)

	// 1. Resolve builder configuration.
	bucket, prefix, err := backend.GetProjectS3Config(ctx, cbClient, plan.BuilderID.ValueString())
	if err != nil {
		diags.AddError("Failed to read builder configuration", err.Error())
		return
	}

	// 2. Prepare context.
	contextKey, contextHash, cleanup, buildDiags := r.prepareAndUploadContext(ctx, s3Client, bucket, prefix, plan)
	diags.Append(buildDiags...)
	if diags.HasError() {
		return
	}
	if cleanup != nil {
		defer cleanup()
	}
	plan.ContextHash = types.StringValue(contextHash)

	// 3. Resolve build parameters.
	dockerfile := "Dockerfile"
	target := ""
	if plan.Definition != nil {
		if !plan.Definition.Dockerfile.IsNull() && !plan.Definition.Dockerfile.IsUnknown() {
			dockerfile = plan.Definition.Dockerfile.ValueString()
		}
		if !plan.Definition.Target.IsNull() && !plan.Definition.Target.IsUnknown() {
			target = plan.Definition.Target.ValueString()
		}
	}

	var platforms []string
	diags.Append(plan.Platforms.ElementsAs(ctx, &platforms, false)...)
	if diags.HasError() {
		return
	}

	var tags []string
	diags.Append(plan.Tags.ElementsAs(ctx, &tags, false)...)
	if diags.HasError() {
		return
	}

	var buildArgs map[string]string
	if !plan.BuildArgs.IsNull() && !plan.BuildArgs.IsUnknown() {
		buildArgs = make(map[string]string)
		diags.Append(plan.BuildArgs.ElementsAs(ctx, &buildArgs, false)...)
		if diags.HasError() {
			return
		}
	}

	var labels map[string]string
	if !plan.Labels.IsNull() && !plan.Labels.IsUnknown() {
		labels = make(map[string]string)
		diags.Append(plan.Labels.ElementsAs(ctx, &labels, false)...)
		if diags.HasError() {
			return
		}
	}

	// 4. Determine result metadata location.
	resultKey := prefix + "results/" + plan.ID.ValueString() + ".json"

	// 5. Start build.
	buildReq := &backend.BuildRequest{
		ProjectName:   plan.BuilderID.ValueString(),
		ContextBucket: bucket,
		ContextKey:    contextKey,
		Dockerfile:    dockerfile,
		Target:        target,
		Platforms:     platforms,
		Tags:          tags,
		BuildArgs:     buildArgs,
		Labels:        labels,
		ResultBucket:  bucket,
		ResultKey:     resultKey,
	}

	tflog.Info(ctx, "starting remote build", map[string]interface{}{
		"project":   plan.BuilderID.ValueString(),
		"context":   contextKey,
		"platforms": strings.Join(platforms, ","),
		"tags":      strings.Join(tags, ","),
	})

	buildID, err := backend.StartBuild(ctx, cbClient, buildReq)
	if err != nil {
		diags.AddError("Failed to start build", err.Error())
		return
	}
	plan.BuildID = types.StringValue(buildID)

	tflog.Info(ctx, "waiting for build to complete", map[string]interface{}{"build_id": buildID})

	// 6. Wait for completion.
	result, err := backend.WaitForBuild(ctx, cbClient, buildID)
	if err != nil {
		if result != nil {
			plan.BuildStatus = types.StringValue(result.Status)
			plan.BuildStartTime = types.StringValue(result.StartTime)
			plan.BuildEndTime = types.StringValue(result.EndTime)
			plan.LogURL = types.StringValue(result.LogURL)
		}
		diags.AddError("Build failed", err.Error())
		return
	}

	plan.BuildStatus = types.StringValue(result.Status)
	plan.BuildStartTime = types.StringValue(result.StartTime)
	plan.BuildEndTime = types.StringValue(result.EndTime)
	plan.LogURL = types.StringValue(result.LogURL)

	// 7. Read result metadata from S3.
	meta, err := backend.ReadResultMetadata(ctx, s3Client, bucket, resultKey)
	if err != nil {
		diags.AddError("Failed to read build result", fmt.Sprintf(
			"Build succeeded but could not read result metadata from s3://%s/%s: %s",
			bucket, resultKey, err,
		))
		return
	}

	plan.ImageDigest = types.StringValue(meta.Digest)
	plan.ImageRef = types.StringValue(meta.ImageRef)
	// MVP: image_index_digest is empty for single-platform builds.
	plan.ImageIndexDigest = types.StringValue("")

	tflog.Info(ctx, "build completed", map[string]interface{}{
		"build_id": buildID,
		"digest":   meta.Digest,
	})
}

func (r *buildResource) prepareAndUploadContext(
	ctx context.Context,
	s3Client *s3.Client,
	bucket, prefix string,
	plan *buildResourceModel,
) (contextKey, contextHash string, cleanup func(), diags diag.Diagnostics) {

	ctxType := buildcontext.ContextType(plan.Context.Type.ValueString())

	switch ctxType {
	case buildcontext.ContextTypeLocal:
		if plan.Context.Path.IsNull() || plan.Context.Path.IsUnknown() {
			diags.AddError("Missing context path", "The 'path' attribute is required when context type is 'local_directory'.")
			return
		}

		prepared, err := buildcontext.PrepareLocalContext(plan.Context.Path.ValueString())
		if err != nil {
			diags.AddError("Failed to prepare build context", err.Error())
			return
		}

		contextHash = prepared.Hash
		contextKey = prefix + "contexts/" + contextHash + ".tar.gz"
		cleanup = func() { _ = os.Remove(prepared.ArchivePath) }

		tflog.Debug(ctx, "uploading context archive", map[string]interface{}{
			"hash": contextHash,
			"size": prepared.Size,
			"key":  contextKey,
		})

		if err := backend.UploadContext(ctx, s3Client, bucket, contextKey, prepared.ArchivePath); err != nil {
			diags.AddError("Failed to upload build context", err.Error())
			return
		}

	case buildcontext.ContextTypeS3:
		if plan.Context.Bucket.IsNull() || plan.Context.Key.IsNull() {
			diags.AddError("Missing S3 context", "Both 'bucket' and 'key' are required when context type is 's3'.")
			return
		}
		// S3 context is already uploaded; use it directly.
		contextKey = plan.Context.Key.ValueString()
		// Use the S3 key as a stand-in hash.
		contextHash = plan.Context.Bucket.ValueString() + "/" + contextKey

	case buildcontext.ContextTypeGit:
		if plan.Context.URL.IsNull() {
			diags.AddError("Missing Git URL", "The 'url' attribute is required when context type is 'git'.")
			return
		}
		// For git context, CodeBuild can pull directly. We encode this as a
		// special source override. For MVP, we set contextKey to a sentinel
		// and the buildspec will handle the git clone.
		diags.AddError("Not yet implemented", "Git context support is not implemented in this MVP. Use 'local_directory' or 's3'.")
		return

	case buildcontext.ContextTypeTarball:
		diags.AddError("Not yet implemented", "Tarball context support is not implemented in this MVP. Use 'local_directory' or 's3'.")
		return

	default:
		diags.AddError("Invalid context type",
			fmt.Sprintf("Unknown context type %q. Valid types: %v", ctxType, buildcontext.ValidContextTypes()))
		return
	}

	return
}

// --- Helpers ---

func generateID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return fmt.Sprintf("%02x%02x%02x%02x-%02x%02x-%02x%02x-%02x%02x-%02x%02x%02x%02x%02x%02x",
		b[0], b[1], b[2], b[3], b[4], b[5], b[6], b[7],
		b[8], b[9], b[10], b[11], b[12], b[13], b[14], b[15])
}
