// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package cmd contains the CLI commands for Zarf.
package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/defenseunicorns/zarf/src/config/lang"
	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/defenseunicorns/zarf/src/types"
	"github.com/pterm/pterm"

	"github.com/AlecAivazis/survey/v2"
	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/internal/cluster"
	"github.com/defenseunicorns/zarf/src/pkg/packager"
	"github.com/defenseunicorns/zarf/src/pkg/utils"
	"github.com/mholt/archiver/v3"
	"github.com/spf13/cobra"
)

var includeInspectSBOM bool
var outputInspectSBOM string

var packageCmd = &cobra.Command{
	Use:     "package",
	Aliases: []string{"p"},
	Short:   lang.CmdPackageShort,
}

var packageCreateCmd = &cobra.Command{
	Use:     "create [DIRECTORY]",
	Aliases: []string{"c"},
	Args:    cobra.MaximumNArgs(1),
	Short:   lang.CmdPackageCreateShort,
	Long:    lang.CmdPackageCreateLong,
	Run: func(cmd *cobra.Command, args []string) {

		var baseDir string

		// If a directory was provided, use that as the base directory
		if len(args) > 0 {
			baseDir = args[0]
		}

		var isCleanPathRegex = regexp.MustCompile(`^[a-zA-Z0-9\_\-\/\.\~\\:]+$`)
		if !isCleanPathRegex.MatchString(config.CommonOptions.CachePath) {
			message.Warnf("Invalid characters in Zarf cache path, defaulting to %s", config.ZarfDefaultCachePath)
			config.CommonOptions.CachePath = config.ZarfDefaultCachePath
		}

		// Ensure uppercase keys from viper
		viperConfig := utils.TransformMapKeys(v.GetStringMapString(V_PKG_CREATE_SET), strings.ToUpper)
		pkgConfig.CreateOpts.SetVariables = utils.MergeMap(viperConfig, pkgConfig.CreateOpts.SetVariables)

		// Configure the packager
		pkgClient := packager.NewOrDie(&pkgConfig)
		defer pkgClient.ClearTempPaths()

		// Create the package
		if err := pkgClient.Create(baseDir); err != nil {
			message.Fatalf(err, "Failed to create package: %s", err.Error())
		}
	},
}

var packageDeployCmd = &cobra.Command{
	Use:     "deploy [PACKAGE]",
	Aliases: []string{"d"},
	Short:   lang.CmdPackageDeployShort,
	Long:    lang.CmdPackageDeployLong,
	Args:    cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		pkgConfig.DeployOpts.PackagePath = choosePackage(args)

		// Ensure uppercase keys from viper
		viperConfig := utils.TransformMapKeys(v.GetStringMapString(V_PKG_DEPLOY_SET), strings.ToUpper)
		pkgConfig.DeployOpts.SetVariables = utils.MergeMap(viperConfig, pkgConfig.DeployOpts.SetVariables)

		// Configure the packager
		pkgClient := packager.NewOrDie(&pkgConfig)
		defer pkgClient.ClearTempPaths()

		// Deploy the package
		if err := pkgClient.Deploy(); err != nil {
			message.Fatalf(err, "Failed to deploy package: %s", err.Error())
		}
	},
}

var packageInspectCmd = &cobra.Command{
	Use:     "inspect [PACKAGE]",
	Aliases: []string{"i"},
	Short:   lang.CmdPackageInspectShort,
	Long:    lang.CmdPackageInspectLong,
	Args:    cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		pkgConfig.DeployOpts.PackagePath = choosePackage(args)

		// Configure the packager
		pkgClient := packager.NewOrDie(&pkgConfig)
		defer pkgClient.ClearTempPaths()

		// Inspect the package
		if err := pkgClient.Inspect(includeInspectSBOM, outputInspectSBOM); err != nil {
			message.Fatalf(err, "Failed to inspect package: %s", err.Error())
		}
	},
}

var packageListCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"l"},
	Short:   lang.CmdPackageListShort,
	Run: func(cmd *cobra.Command, args []string) {
		// Get all the deployed packages
		deployedZarfPackages, err := cluster.NewClusterOrDie().GetDeployedZarfPackages()
		if err != nil {
			message.Fatalf(err, lang.CmdPackageListNoPackageWarn)
		}

		// Populate a pterm table of all the deployed packages
		packageTable := pterm.TableData{
			{"     Package ", "Components"},
		}

		for _, pkg := range deployedZarfPackages {
			var components []string

			for _, component := range pkg.DeployedComponents {
				components = append(components, component.Name)
			}

			packageTable = append(packageTable, pterm.TableData{{
				fmt.Sprintf("     %s", pkg.Name),
				fmt.Sprintf("%v", components),
			}}...)
		}

		// Print out the table for the user
		_ = pterm.DefaultTable.WithHasHeader().WithData(packageTable).Render()
	},
}

var packageRemoveCmd = &cobra.Command{
	Use:     "remove {PACKAGE_NAME|PACKAGE_FILE}",
	Aliases: []string{"u"},
	Args:    cobra.ExactArgs(1),
	Short:   lang.CmdPackageRemoveShort,
	Run: func(cmd *cobra.Command, args []string) {
		pkgName := args[0]

		// If the user input is a path to a package, extract the name from the package
		isTarball := regexp.MustCompile(`.*zarf-package-.*\.tar\.zst$`).MatchString
		if isTarball(pkgName) {
			if utils.InvalidPath(pkgName) {
				message.Fatalf(nil, lang.CmdPackageRemoveTarballErr)
			}

			tempPath, err := utils.MakeTempDir(config.CommonOptions.TempDirectory)
			if err != nil {
				message.Fatalf(err, "Unable to create tmpdir: %s", config.CommonOptions.TempDirectory)
			}
			defer os.RemoveAll(tempPath)

			if err := archiver.Extract(pkgName, config.ZarfYAML, tempPath); err != nil {
				message.Fatalf(err, lang.CmdPackageRemoveExtractErr)
			}

			var pkgConfig types.ZarfPackage
			configPath := filepath.Join(tempPath, config.ZarfYAML)
			if err := utils.ReadYaml(configPath, &pkgConfig); err != nil {
				message.Fatalf(err, lang.CmdPackageRemoveReadZarfErr)
			}

			pkgName = pkgConfig.Metadata.Name
		}

		// Configure the packager
		pkgClient := packager.NewOrDie(&pkgConfig)
		defer pkgClient.ClearTempPaths()

		if err := pkgClient.Remove(pkgName); err != nil {
			message.Fatalf(err, "Unable to remove the package with an error of: %#v", err)
		}
	},
}

func choosePackage(args []string) string {
	if len(args) > 0 {
		return args[0]
	}
	var path string
	prompt := &survey.Input{
		Message: "Choose or type the package file",
		Suggest: func(toComplete string) []string {
			files, _ := filepath.Glob(config.ZarfPackagePrefix + toComplete + "*.tar")
			gzFiles, _ := filepath.Glob(config.ZarfPackagePrefix + toComplete + "*.tar.zst")
			partialFiles, _ := filepath.Glob(config.ZarfPackagePrefix + toComplete + "*.part000")

			files = append(files, gzFiles...)
			files = append(files, partialFiles...)
			return files
		},
	}

	if err := survey.AskOne(prompt, &path, survey.WithValidator(survey.Required)); err != nil {
		message.Fatalf(nil, "Package path selection canceled: %s", err.Error())
	}

	return path
}

func init() {
	initViper()

	rootCmd.AddCommand(packageCmd)
	packageCmd.AddCommand(packageCreateCmd)
	packageCmd.AddCommand(packageDeployCmd)
	packageCmd.AddCommand(packageInspectCmd)
	packageCmd.AddCommand(packageRemoveCmd)
	packageCmd.AddCommand(packageListCmd)

	bindCreateFlags()
	bindDeployFlags()
	bindInspectFlags()
	bindRemoveFlags()
}

func bindCreateFlags() {
	createFlags := packageCreateCmd.Flags()

	// Always require confirm flag (no viper)
	createFlags.BoolVar(&config.CommonOptions.Confirm, "confirm", false, lang.CmdPackageCreateFlagConfirm)

	v.SetDefault(V_PKG_CREATE_SET, map[string]string{})
	v.SetDefault(V_PKG_CREATE_OUTPUT_DIR, "")
	v.SetDefault(V_PKG_CREATE_SBOM, false)
	v.SetDefault(V_PKG_CREATE_SBOM_OUTPUT, "")
	v.SetDefault(V_PKG_CREATE_SKIP_SBOM, false)
	v.SetDefault(V_PKG_CREATE_MAX_PACKAGE_SIZE, 0)

	createFlags.StringToStringVar(&pkgConfig.CreateOpts.SetVariables, "set", v.GetStringMapString(V_PKG_CREATE_SET), lang.CmdPackageCreateFlagSet)
	createFlags.StringVarP(&pkgConfig.CreateOpts.OutputDirectory, "output-directory", "o", v.GetString(V_PKG_CREATE_OUTPUT_DIR), lang.CmdPackageCreateFlagOutputDirectory)
	createFlags.BoolVarP(&pkgConfig.CreateOpts.ViewSBOM, "sbom", "s", v.GetBool(V_PKG_CREATE_SBOM), lang.CmdPackageCreateFlagSbom)
	createFlags.StringVar(&pkgConfig.CreateOpts.SBOMOutputDir, "sbom-out", v.GetString(V_PKG_CREATE_SBOM_OUTPUT), lang.CmdPackageCreateFlagSbomOut)
	createFlags.BoolVar(&pkgConfig.CreateOpts.SkipSBOM, "skip-sbom", v.GetBool(V_PKG_CREATE_SKIP_SBOM), lang.CmdPackageCreateFlagSkipSbom)
	createFlags.IntVarP(&pkgConfig.CreateOpts.MaxPackageSizeMB, "max-package-size", "m", v.GetInt(V_PKG_CREATE_MAX_PACKAGE_SIZE), lang.CmdPackageCreateFlagMaxPackageSize)
}

func bindDeployFlags() {
	deployFlags := packageDeployCmd.Flags()

	// Always require confirm flag (no viper)
	deployFlags.BoolVar(&config.CommonOptions.Confirm, "confirm", false, lang.CmdPackageDeployFlagConfirm)

	v.SetDefault(V_PKG_DEPLOY_SET, map[string]string{})
	v.SetDefault(V_PKG_DEPLOY_COMPONENTS, "")
	v.SetDefault(V_PKG_DEPLOY_SHASUM, "")
	v.SetDefault(V_PKG_DEPLOY_SGET, "")

	deployFlags.StringToStringVar(&pkgConfig.DeployOpts.SetVariables, "set", v.GetStringMapString(V_PKG_DEPLOY_SET), lang.CmdPackageDeployFlagSet)
	deployFlags.StringVar(&pkgConfig.DeployOpts.Components, "components", v.GetString(V_PKG_DEPLOY_COMPONENTS), lang.CmdPackageDeployFlagComponents)
	deployFlags.StringVar(&pkgConfig.DeployOpts.Shasum, "shasum", v.GetString(V_PKG_DEPLOY_SHASUM), lang.CmdPackageDeployFlagShasum)
	deployFlags.StringVar(&pkgConfig.DeployOpts.SGetKeyPath, "sget", v.GetString(V_PKG_DEPLOY_SGET), lang.CmdPackageDeployFlagSget)
}

func bindInspectFlags() {
	inspectFlags := packageInspectCmd.Flags()
	inspectFlags.BoolVarP(&includeInspectSBOM, "sbom", "s", false, lang.CmdPackageInspectFlagSbom)
	inspectFlags.StringVar(&outputInspectSBOM, "sbom-out", "", lang.CmdPackageInspectFlagSbomOut)
}

func bindRemoveFlags() {
	removeFlags := packageRemoveCmd.Flags()
	removeFlags.BoolVar(&config.CommonOptions.Confirm, "confirm", false, lang.CmdPackageRemoveFlagConfirm)
	removeFlags.StringVar(&pkgConfig.DeployOpts.Components, "components", v.GetString(V_PKG_DEPLOY_COMPONENTS), lang.CmdPackageRemoveFlagComponents)
	_ = packageRemoveCmd.MarkFlagRequired("confirm")
}
