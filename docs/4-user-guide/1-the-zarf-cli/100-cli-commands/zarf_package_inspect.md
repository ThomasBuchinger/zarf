# zarf package inspect
<!-- Auto-generated by docs/gen-cli-docs.sh -->

Lists the payload of a Zarf package (runs offline)

## Synopsis

Lists the payload of a compiled package file (runs offline)
Unpacks the package tarball into a temp directory and displays the contents of the archive.

```
zarf package inspect [PACKAGE] [flags]
```

## Options

```
  -h, --help              help for inspect
  -s, --sbom              View SBOM contents while inspecting the package
      --sbom-out string   Specify an output directory for the SBOMs from the inspected Zarf package
```

## Options inherited from parent commands

```
  -a, --architecture string   Architecture for OCI images
      --insecure              Allow access to insecure registries and disable other recommended security enforcements. This flag should only be used if you have a specific reason and accept the reduced security posture.
  -l, --log-level string      Log level when running Zarf. Valid options are: warn, info, debug, trace (default "info")
      --no-log-file           Disable log file creation
      --no-progress           Disable fancy UI progress bars, spinners, logos, etc
      --tmpdir string         Specify the temporary directory to use for intermediate files
      --zarf-cache string     Specify the location of the Zarf cache directory (default "~/.zarf-cache")
```

## SEE ALSO

* [zarf package](zarf_package.md)	 - Zarf package commands for creating, deploying, and inspecting packages

