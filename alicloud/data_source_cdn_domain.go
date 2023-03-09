package alicloud

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/types"

	alicloudCdnClient "github.com/alibabacloud-go/cdn-20180510/v2/client"
	util "github.com/alibabacloud-go/tea-utils/v2/service"
	"github.com/alibabacloud-go/tea/tea"
)

var (
	_ datasource.DataSource              = &cdnDomainDataSource{}
	_ datasource.DataSourceWithConfigure = &cdnDomainDataSource{}
)

func NewCdnDomainDataSource() datasource.DataSource {
	return &cdnDomainDataSource{}
}

type cdnDomainDataSource struct {
	client *alicloudCdnClient.Client
}

type cdnDomainDataSourceModel struct {
	DomainName  types.String `tfsdk:"domain_name"`
	DomainCName types.String `tfsdk:"domain_cname"`
	Origins     types.List   `tfsdk:"origins"`
}

func (d *cdnDomainDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_cdn_domain"
}

func (d *cdnDomainDataSource) Schema(_ context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "This data source provides the CDN Instance of the current Alibaba Cloud user.",
		Attributes: map[string]schema.Attribute{
			"domain_name": schema.StringAttribute{
				Description: "Domain name of CDN domain.",
				Required:    true,
			},
			"domain_cname": schema.StringAttribute{
				Description: "Domain CName of CDN domain.",
				Computed:    true,
			},
			"origins": schema.ListAttribute{
				Description: "Origins of CDN domain.",
				ElementType: types.StringType,
				Computed:    true,
			},
		},
	}
}

func (d *cdnDomainDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	d.client = req.ProviderData.(alicloudClients).cdnClient
}

func (d *cdnDomainDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var plan, state cdnDomainDataSourceModel
	diags := req.Config.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	domainName := plan.DomainName.ValueString()

	if domainName == "" {
		resp.Diagnostics.AddAttributeError(
			path.Root("domain_name"),
			"Missing CDN Domain Name",
			"Domain name must not be empty",
		)
		return
	}

	var cdnDomains *alicloudCdnClient.DescribeCdnDomainDetailResponse
	var err error
	describeCdnDomainDetailRequest := &alicloudCdnClient.DescribeCdnDomainDetailRequest{
		DomainName: tea.String(domainName),
	}
	runtime := &util.RuntimeOptions{}
	tryErr := func() (_e error) {
		defer func() {
			if r := tea.Recover(recover()); r != nil {
				_e = r
			}
		}()

		cdnDomains, err = d.client.DescribeCdnDomainDetailWithOptions(describeCdnDomainDetailRequest, runtime)
		if err != nil {
			return err
		}

		return nil
	}()

	if tryErr != nil {
		var error = &tea.SDKError{}
		if _t, ok := tryErr.(*tea.SDKError); ok {
			error = _t
		} else {
			error.Message = tea.String(tryErr.Error())
		}

		_, err := util.AssertAsString(error.Message)
		if err != nil {
			resp.Diagnostics.AddError(
				"[API ERROR] Failed to Query CDN Domains",
				err.Error(),
			)
			return
		}
	}

	if cdnDomains.String() != "{}" {
		state.DomainName = types.StringValue(*cdnDomains.Body.GetDomainDetailModel.DomainName)
		state.DomainCName = types.StringValue(*cdnDomains.Body.GetDomainDetailModel.Cname)
		var originsRaw []string
		for _, sourceModel := range cdnDomains.Body.GetDomainDetailModel.SourceModels.SourceModel {
			originsRaw = append(originsRaw, *sourceModel.Content)
		}
		originsList, diags := types.ListValueFrom(ctx, types.StringType, originsRaw)
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}
		state.Origins = originsList
	} else {
		state.DomainName = types.StringNull()
		state.DomainCName = types.StringNull()
		state.Origins = types.ListNull(types.StringType)
	}

	diags = resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}
