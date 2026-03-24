package provider

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ provider.Provider = &wharflabProvider{}

// ProviderData holds configured clients passed to resources and data sources.
type ProviderData struct {
	AWSConfig aws.Config
}

type wharflabProvider struct {
	version string
}

type wharflabProviderModel struct {
	Region    types.String `tfsdk:"region"`
	AccessKey types.String `tfsdk:"access_key"`
	SecretKey types.String `tfsdk:"secret_key"`
	Profile   types.String `tfsdk:"profile"`
}

// New returns a factory function for the WharfLab provider.
func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &wharflabProvider{version: version}
	}
}

func (p *wharflabProvider) Metadata(_ context.Context, _ provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "containers"
	resp.Version = p.version
}

func (p *wharflabProvider) Schema(_ context.Context, _ provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "WharfLab Containers provider for remote container builds and OCI artifact production.",
		Attributes: map[string]schema.Attribute{
			"region": schema.StringAttribute{
				Optional:    true,
				Description: "AWS region for backend services. Can also be set via AWS_REGION or AWS_DEFAULT_REGION environment variables.",
			},
			"access_key": schema.StringAttribute{
				Optional:    true,
				Sensitive:   true,
				Description: "AWS access key ID. Can also be set via AWS_ACCESS_KEY_ID environment variable.",
			},
			"secret_key": schema.StringAttribute{
				Optional:    true,
				Sensitive:   true,
				Description: "AWS secret access key. Can also be set via AWS_SECRET_ACCESS_KEY environment variable.",
			},
			"profile": schema.StringAttribute{
				Optional:    true,
				Description: "AWS profile name for credential resolution.",
			},
		},
	}
}

func (p *wharflabProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var data wharflabProviderModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var opts []func(*awsconfig.LoadOptions) error

	if !data.Region.IsNull() && !data.Region.IsUnknown() {
		opts = append(opts, awsconfig.WithRegion(data.Region.ValueString()))
	}

	if !data.Profile.IsNull() && !data.Profile.IsUnknown() {
		opts = append(opts, awsconfig.WithSharedConfigProfile(data.Profile.ValueString()))
	}

	if !data.AccessKey.IsNull() && !data.SecretKey.IsNull() &&
		!data.AccessKey.IsUnknown() && !data.SecretKey.IsUnknown() {
		opts = append(opts, awsconfig.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(
				data.AccessKey.ValueString(),
				data.SecretKey.ValueString(),
				"",
			),
		))
	}

	cfg, err := awsconfig.LoadDefaultConfig(ctx, opts...)
	if err != nil {
		resp.Diagnostics.AddError("Failed to configure AWS", err.Error())
		return
	}

	providerData := &ProviderData{AWSConfig: cfg}
	resp.DataSourceData = providerData
	resp.ResourceData = providerData
}

func (p *wharflabProvider) DataSources(_ context.Context) []func() datasource.DataSource {
	return nil
}

func (p *wharflabProvider) Resources(_ context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		NewAWSCodeBuildBuilderResource,
		NewBuildResource,
	}
}
