package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/codebuild"
	cbtypes "github.com/aws/aws-sdk-go-v2/service/codebuild/types"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.Resource                = &awsCodeBuildBuilderResource{}
	_ resource.ResourceWithConfigure   = &awsCodeBuildBuilderResource{}
	_ resource.ResourceWithImportState = &awsCodeBuildBuilderResource{}
)

type awsCodeBuildBuilderResource struct {
	providerData *ProviderData
}

// --- Models ---

type awsCodeBuildBuilderModel struct {
	ID             types.String              `tfsdk:"id"`
	Name           types.String              `tfsdk:"name"`
	ProjectName    types.String              `tfsdk:"project_name"`
	ServiceRoleARN types.String              `tfsdk:"service_role_arn"`
	ArtifactBucket types.String              `tfsdk:"artifact_bucket"`
	ArtifactPrefix types.String              `tfsdk:"artifact_prefix"`
	Region         types.String              `tfsdk:"region"`
	Environment    *builderEnvironmentModel  `tfsdk:"environment"`
	Logging        *builderLoggingModel      `tfsdk:"logging"`
	VPCConfig      *builderVPCConfigModel    `tfsdk:"vpc_config"`
	Tags           types.Map                 `tfsdk:"tags"`
	ProjectARN     types.String              `tfsdk:"project_arn"`
}

type builderEnvironmentModel struct {
	Image       types.String `tfsdk:"image"`
	ComputeType types.String `tfsdk:"compute_type"`
	Privileged  types.Bool   `tfsdk:"privileged"`
}

type builderLoggingModel struct {
	CloudWatchEnabled types.Bool   `tfsdk:"cloudwatch_enabled"`
	LogGroupName      types.String `tfsdk:"log_group_name"`
	StreamName        types.String `tfsdk:"stream_name"`
}

type builderVPCConfigModel struct {
	VPCID            types.String `tfsdk:"vpc_id"`
	Subnets          types.List   `tfsdk:"subnets"`
	SecurityGroupIDs types.List   `tfsdk:"security_group_ids"`
}

// --- Constructor ---

func NewAWSCodeBuildBuilderResource() resource.Resource {
	return &awsCodeBuildBuilderResource{}
}

// --- Metadata ---

func (r *awsCodeBuildBuilderResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_aws_codebuild_builder"
}

// --- Schema ---

func (r *awsCodeBuildBuilderResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages an AWS CodeBuild project configured as a remote container build backend.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "Builder identifier (CodeBuild project name).",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				Required:    true,
				Description: "Logical name for this builder.",
			},
			"project_name": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "CodeBuild project name. Defaults to 'wharflab-{name}'.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"service_role_arn": schema.StringAttribute{
				Required:    true,
				Description: "IAM service role ARN for the CodeBuild project.",
			},
			"artifact_bucket": schema.StringAttribute{
				Required:    true,
				Description: "S3 bucket for build context uploads and build metadata.",
			},
			"artifact_prefix": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Default:     stringdefault.StaticString("wharflab/"),
				Description: "S3 key prefix for build artifacts. Defaults to 'wharflab/'.",
			},
			"region": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "AWS region override. Defaults to the provider region.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"tags": schema.MapAttribute{
				Optional:    true,
				ElementType: types.StringType,
				Description: "Tags to apply to the CodeBuild project.",
			},
			"project_arn": schema.StringAttribute{
				Computed:    true,
				Description: "ARN of the CodeBuild project.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
		},
		Blocks: map[string]schema.Block{
			"environment": schema.SingleNestedBlock{
				Description: "Build environment configuration. Required.",
				Attributes: map[string]schema.Attribute{
					"image": schema.StringAttribute{
						Required:    true,
						Description: "Docker image for the build environment (e.g. 'aws/codebuild/standard:7.0').",
					},
					"compute_type": schema.StringAttribute{
						Required:    true,
						Description: "Compute type (e.g. 'BUILD_GENERAL1_SMALL', 'BUILD_GENERAL1_MEDIUM', 'BUILD_GENERAL1_LARGE').",
					},
					"privileged": schema.BoolAttribute{
						Optional:    true,
						Computed:    true,
						Default:     booldefault.StaticBool(true),
						Description: "Enable privileged mode for Docker-in-Docker builds. Defaults to true.",
					},
				},
			},
			"logging": schema.SingleNestedBlock{
				Description: "CloudWatch logging configuration.",
				Attributes: map[string]schema.Attribute{
					"cloudwatch_enabled": schema.BoolAttribute{
						Optional:    true,
						Computed:    true,
						Default:     booldefault.StaticBool(true),
						Description: "Enable CloudWatch logging. Defaults to true.",
					},
					"log_group_name": schema.StringAttribute{
						Optional:    true,
						Description: "CloudWatch log group name.",
					},
					"stream_name": schema.StringAttribute{
						Optional:    true,
						Description: "CloudWatch log stream name prefix.",
					},
				},
			},
			"vpc_config": schema.SingleNestedBlock{
				Description: "VPC configuration for the build environment.",
				Attributes: map[string]schema.Attribute{
					"vpc_id": schema.StringAttribute{
						Required:    true,
						Description: "VPC ID.",
					},
					"subnets": schema.ListAttribute{
						Required:    true,
						ElementType: types.StringType,
						Description: "List of subnet IDs.",
					},
					"security_group_ids": schema.ListAttribute{
						Required:    true,
						ElementType: types.StringType,
						Description: "List of security group IDs.",
					},
				},
			},
		},
	}
}

// --- Configure ---

func (r *awsCodeBuildBuilderResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

// --- CRUD ---

func (r *awsCodeBuildBuilderResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan awsCodeBuildBuilderModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if plan.Environment == nil {
		resp.Diagnostics.AddError("Missing required block", "The 'environment' block is required.")
		return
	}

	// Resolve defaults.
	projectName := plan.ProjectName.ValueString()
	if plan.ProjectName.IsNull() || plan.ProjectName.IsUnknown() {
		projectName = "wharflab-" + plan.Name.ValueString()
	}

	cfg := r.awsConfig(plan.Region)
	client := codebuild.NewFromConfig(cfg)

	// Build the S3 source location: "bucket/prefix" — used by build resources
	// to discover where to upload context archives.
	sourceLocation := plan.ArtifactBucket.ValueString() + "/" + plan.ArtifactPrefix.ValueString()

	input := &codebuild.CreateProjectInput{
		Name: aws.String(projectName),
		Source: &cbtypes.ProjectSource{
			Type:      cbtypes.SourceTypeS3,
			Location:  aws.String(sourceLocation),
			Buildspec: aws.String("version: 0.2\nphases:\n  build:\n    commands:\n      - echo placeholder"),
		},
		Artifacts: &cbtypes.ProjectArtifacts{
			Type: cbtypes.ArtifactsTypeNoArtifacts,
		},
		Environment: r.buildEnvironment(plan.Environment),
		ServiceRole: aws.String(plan.ServiceRoleARN.ValueString()),
	}

	if plan.Logging != nil {
		input.LogsConfig = r.buildLogsConfig(plan.Logging)
	}

	if plan.VPCConfig != nil {
		vpcCfg, diags := r.buildVPCConfig(ctx, plan.VPCConfig)
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}
		input.VpcConfig = vpcCfg
	}

	if !plan.Tags.IsNull() {
		tags, diags := r.buildTags(ctx, plan.Tags)
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}
		input.Tags = tags
	}

	output, err := client.CreateProject(ctx, input)
	if err != nil {
		resp.Diagnostics.AddError("Failed to create CodeBuild project", err.Error())
		return
	}

	plan.ID = types.StringValue(projectName)
	plan.ProjectName = types.StringValue(projectName)
	plan.Region = types.StringValue(cfg.Region)
	plan.ProjectARN = types.StringValue(aws.ToString(output.Project.Arn))

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *awsCodeBuildBuilderResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state awsCodeBuildBuilderModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	cfg := r.awsConfig(state.Region)
	client := codebuild.NewFromConfig(cfg)

	output, err := client.BatchGetProjects(ctx, &codebuild.BatchGetProjectsInput{
		Names: []string{state.ProjectName.ValueString()},
	})
	if err != nil {
		resp.Diagnostics.AddError("Failed to read CodeBuild project", err.Error())
		return
	}
	if len(output.Projects) == 0 {
		resp.State.RemoveResource(ctx)
		return
	}

	project := output.Projects[0]
	state.ProjectARN = types.StringValue(aws.ToString(project.Arn))
	state.ServiceRoleARN = types.StringValue(aws.ToString(project.ServiceRole))

	if project.Environment != nil {
		state.Environment = &builderEnvironmentModel{
			Image:       types.StringValue(aws.ToString(project.Environment.Image)),
			ComputeType: types.StringValue(string(project.Environment.ComputeType)),
			Privileged:  types.BoolValue(aws.ToBool(project.Environment.PrivilegedMode)),
		}
	}

	// Parse bucket/prefix back from source location.
	if project.Source != nil && project.Source.Location != nil {
		loc := aws.ToString(project.Source.Location)
		if parts := strings.SplitN(loc, "/", 2); len(parts) == 2 {
			state.ArtifactBucket = types.StringValue(parts[0])
			state.ArtifactPrefix = types.StringValue(parts[1])
		}
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *awsCodeBuildBuilderResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan awsCodeBuildBuilderModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if plan.Environment == nil {
		resp.Diagnostics.AddError("Missing required block", "The 'environment' block is required.")
		return
	}

	cfg := r.awsConfig(plan.Region)
	client := codebuild.NewFromConfig(cfg)

	sourceLocation := plan.ArtifactBucket.ValueString() + "/" + plan.ArtifactPrefix.ValueString()

	input := &codebuild.UpdateProjectInput{
		Name: aws.String(plan.ProjectName.ValueString()),
		Source: &cbtypes.ProjectSource{
			Type:      cbtypes.SourceTypeS3,
			Location:  aws.String(sourceLocation),
			Buildspec: aws.String("version: 0.2\nphases:\n  build:\n    commands:\n      - echo placeholder"),
		},
		Artifacts: &cbtypes.ProjectArtifacts{
			Type: cbtypes.ArtifactsTypeNoArtifacts,
		},
		Environment: r.buildEnvironment(plan.Environment),
		ServiceRole: aws.String(plan.ServiceRoleARN.ValueString()),
	}

	if plan.Logging != nil {
		input.LogsConfig = r.buildLogsConfig(plan.Logging)
	}

	if plan.VPCConfig != nil {
		vpcCfg, diags := r.buildVPCConfig(ctx, plan.VPCConfig)
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}
		input.VpcConfig = vpcCfg
	}

	if !plan.Tags.IsNull() {
		tags, diags := r.buildTags(ctx, plan.Tags)
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}
		input.Tags = tags
	}

	output, err := client.UpdateProject(ctx, input)
	if err != nil {
		resp.Diagnostics.AddError("Failed to update CodeBuild project", err.Error())
		return
	}

	plan.ProjectARN = types.StringValue(aws.ToString(output.Project.Arn))
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *awsCodeBuildBuilderResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state awsCodeBuildBuilderModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	cfg := r.awsConfig(state.Region)
	client := codebuild.NewFromConfig(cfg)

	_, err := client.DeleteProject(ctx, &codebuild.DeleteProjectInput{
		Name: aws.String(state.ProjectName.ValueString()),
	})
	if err != nil {
		resp.Diagnostics.AddError("Failed to delete CodeBuild project", err.Error())
	}
}

func (r *awsCodeBuildBuilderResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

// --- Helpers ---

func (r *awsCodeBuildBuilderResource) awsConfig(region types.String) aws.Config {
	cfg := r.providerData.AWSConfig
	if !region.IsNull() && !region.IsUnknown() {
		cfg.Region = region.ValueString()
	}
	return cfg
}

func (r *awsCodeBuildBuilderResource) buildEnvironment(env *builderEnvironmentModel) *cbtypes.ProjectEnvironment {
	return &cbtypes.ProjectEnvironment{
		Type:           cbtypes.EnvironmentTypeLinuxContainer,
		Image:          aws.String(env.Image.ValueString()),
		ComputeType:    cbtypes.ComputeType(env.ComputeType.ValueString()),
		PrivilegedMode: aws.Bool(env.Privileged.ValueBool()),
	}
}

func (r *awsCodeBuildBuilderResource) buildLogsConfig(logging *builderLoggingModel) *cbtypes.LogsConfig {
	cw := &cbtypes.CloudWatchLogsConfig{
		Status: cbtypes.LogsConfigStatusTypeEnabled,
	}
	if !logging.CloudWatchEnabled.ValueBool() {
		cw.Status = cbtypes.LogsConfigStatusTypeDisabled
	}
	if !logging.LogGroupName.IsNull() && !logging.LogGroupName.IsUnknown() {
		cw.GroupName = aws.String(logging.LogGroupName.ValueString())
	}
	if !logging.StreamName.IsNull() && !logging.StreamName.IsUnknown() {
		cw.StreamName = aws.String(logging.StreamName.ValueString())
	}
	return &cbtypes.LogsConfig{CloudWatchLogs: cw}
}

func (r *awsCodeBuildBuilderResource) buildVPCConfig(ctx context.Context, vpc *builderVPCConfigModel) (*cbtypes.VpcConfig, diag.Diagnostics) {
	var diags diag.Diagnostics
	var subnets, sgs []string
	diags.Append(vpc.Subnets.ElementsAs(ctx, &subnets, false)...)
	diags.Append(vpc.SecurityGroupIDs.ElementsAs(ctx, &sgs, false)...)

	return &cbtypes.VpcConfig{
		VpcId:            aws.String(vpc.VPCID.ValueString()),
		Subnets:          subnets,
		SecurityGroupIds: sgs,
	}, diags
}

func (r *awsCodeBuildBuilderResource) buildTags(ctx context.Context, tagsMap types.Map) ([]cbtypes.Tag, diag.Diagnostics) {
	var diags diag.Diagnostics
	var m map[string]string
	diags.Append(tagsMap.ElementsAs(ctx, &m, false)...)

	var tags []cbtypes.Tag
	for k, v := range m {
		tags = append(tags, cbtypes.Tag{Key: aws.String(k), Value: aws.String(v)})
	}
	return tags, diags
}
