package vercel

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"os"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/vercel/terraform-provider-vercel/glob"
)

type dataSourceProjectDirectoryType struct{}

func (r dataSourceProjectDirectoryType) GetSchema(_ context.Context) (tfsdk.Schema, diag.Diagnostics) {
	return tfsdk.Schema{
		Description: `
Provides information about files within a directory on disk.

This will recursively read files, providing metadata for use with a deployment.

-> This is intended to be used with the ` + "`vercel_deployment` resource only." + `

-> If you want to prevent files from being included, this can be done with a [vercelignore file](https://vercel.com/guides/prevent-uploading-sourcepaths-with-vercelignore).
        `,
		Attributes: map[string]tfsdk.Attribute{
			"path": {
				Description: "The path to the directory on your filesystem. Note that the path is relative to the root of the terraform files.",
				Required:    true,
				Type:        types.StringType,
			},
			"id": {
				Computed: true,
				Type:     types.StringType,
			},
			"files": {
				Description: "A map of filename to metadata about the file. The metadata contains the file size and hash, and allows a deployment to be created if the file changes.",
				Computed:    true,
				Type: types.MapType{
					ElemType: types.StringType,
				},
			},
		},
	}, nil
}

func (r dataSourceProjectDirectoryType) NewDataSource(ctx context.Context, p tfsdk.Provider) (tfsdk.DataSource, diag.Diagnostics) {
	return dataSourceProjectDirectory{
		p: *(p.(*provider)),
	}, nil
}

type dataSourceProjectDirectory struct {
	p provider
}

type ProjectDirectoryData struct {
	Path  types.String      `tfsdk:"path"`
	ID    types.String      `tfsdk:"id"`
	Files map[string]string `tfsdk:"files"`
}

func (r dataSourceProjectDirectory) Read(ctx context.Context, req tfsdk.ReadDataSourceRequest, resp *tfsdk.ReadDataSourceResponse) {
	var config ProjectDirectoryData
	diags := req.Config.Get(ctx, &config)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	ignoreRules, err := glob.GetIgnores(config.Path.Value)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error reading .vercelignore file",
			fmt.Sprintf("Could not read file, unexpected error: %s",
				err,
			),
		)
		return
	}

	paths, err := glob.GetPaths(config.Path.Value, ignoreRules)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error reading directory",
			fmt.Sprintf("Could not read files for directory %s, unexpected error: %s",
				config.Path.Value,
				err,
			),
		)
		return
	}

	config.Files = map[string]string{}
	for _, path := range paths {
		content, err := os.ReadFile(path)
		if err != nil {
			resp.Diagnostics.AddError(
				"Error reading file",
				fmt.Sprintf("Could not read file %s, unexpected error: %s",
					config.Path.Value,
					err,
				),
			)
			return
		}
		rawSha := sha1.Sum(content)
		sha := hex.EncodeToString(rawSha[:])

		config.Files[path] = fmt.Sprintf("%d~%s", len(content), sha)
	}

	config.ID = config.Path
	diags = resp.State.Set(ctx, &config)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}
