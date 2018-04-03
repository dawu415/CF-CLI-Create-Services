package main

import (
	"fmt"
	"os"
	"strings"

	"code.cloudfoundry.org/cli/plugin"
)

// Create-Service-Push is the struct implementing the interface defined by the core CLI. It can
// be found at  "code.cloudfoundry.org/cli/plugin/plugin.go"
type CreateServicePush struct {
	manifest *Manifest
	cf       plugin.CliConnection
}

// Run must be implemented by any plugin because it is part of the
// plugin interface defined by the core CLI.
//
// Run(....) is the entry point when the core CLI is invoking a command defined
// by a plugin. The first parameter, plugin.CliConnection, is a struct that can
// be used to invoke cli commands. The second paramter, args, is a slice of
// strings. args[0] will be the name of the command, and will be followed by
// any additional arguments a cli user typed in.
//
// Any error handling should be handled with the plugin itself (this means printing
// user facing errors). The CLI will exit 0 if the plugin exits 0 and will exit
// 1 should the plugin exits nonzero.
func (c *CreateServicePush) Run(cliConnection plugin.CliConnection, args []string) {

	if args[0] != "create-service-push" {
		return
	}

	// 1. Find an argument of --service-manifest in the list.  This will tell us the manifest file
	var manifestFilename = "services-manifest.yml"
	var pushApplication = true

	for i, arg := range args {
		if arg == "--service-manifest" {
			manifestFilename = args[i+1]
			break
		} else if arg == "--no-service-manifest" {
			manifestFilename = ""
			break
		}
	}
	// Also check for other specific flags
	for _, arg := range args {
		if arg == "--no-push" {
			pushApplication = false
			break
		}
	}

	// 2. Whatever the manifest file is, check to make sure it exists!
	if len(manifestFilename) > 0 {
		if _, err := os.Stat(manifestFilename); !os.IsNotExist(err) {
			fmt.Printf("Found ManifestFile: %s\n", manifestFilename)
			filePointer, err := os.Open(manifestFilename)
			if err == nil {
				manifest, err := ParseManifest(filePointer)
				if err != nil {
					fmt.Printf("ERROR: %s\n", err)
					os.Exit(1)
				}

				createServicesobject := &CreateServicePush{
					manifest: &manifest,
					cf:       cliConnection,
				}
				createServicesobject.createServices()
			} else {
				fmt.Printf("ERROR: Unable to open %s.\n", manifestFilename)
				os.Exit(1)
			}
		} else {
			fmt.Printf("ERROR: The file %s was not found.\n", manifestFilename)
			os.Exit(1)
		}
	}

	if pushApplication {
		fmt.Printf("Performing a CF Push with arguments %s\n", strings.Join(args[1:], " "))

		newArgs := append([]string{"push"}, args[1:]...)
		// 3. Perform the cf push
		output, err := cliConnection.CliCommand(newArgs...)
		fmt.Printf("%s\n", output)

		if err != nil {
			fmt.Printf("ERROR while pushing: %s\n", err)
		}
	}
}

func (c *CreateServicePush) createServices() error {

	for _, serviceObject := range c.manifest.Services {
		if err := c.createService(serviceObject.ServiceName, serviceObject.Broker, serviceObject.PlanName, serviceObject.JSONParameters); err != nil {
			fmt.Printf("Create Service Error: %+v \n", err)
		}
	}

	return nil
}

func (c *CreateServicePush) run(args ...string) error {
	if os.Getenv("DEBUG") != "" {
		fmt.Printf(">> %s\n", strings.Join(args, " "))
	}

	fmt.Printf("Now Running CLI Command: %s\n", strings.Join(args, " "))
	_, err := c.cf.CliCommand(args...)
	return err
}

func (c *CreateServicePush) createService(name, broker, plan, JSONParam string) error {
	s, err := c.cf.GetServices()
	if err != nil {
		return err
	}

	for _, svc := range s {
		if svc.Name == name {
			fmt.Printf("%s already exists.\n", name)
			return nil
		}
	}

	fmt.Printf("%s will now be created.\n", name)

	var result error
	if JSONParam == "" {
		result = c.run("create-service", broker, plan, name)
	} else {
		result = c.run("create-service", broker, plan, name, "-c", JSONParam)
	}

	if result != nil {
		return result
	}

	pb := NewProgressSpinner(os.Stdout)
	for {
		service, err := c.cf.GetService(name)
		if err != nil {
			return err
		}

		pb.Next(service.LastOperation.Description)

		if service.LastOperation.State == "succeeded" {
			break
		} else if service.LastOperation.State == "failed" {
			return fmt.Errorf(
				"error %s [status: %s]",
				service.LastOperation.Description,
				service.LastOperation.State,
			)
		}

	}

	return nil
}

// GetMetadata must be implemented as part of the plugin interface
// defined by the core CLI.
//
// GetMetadata() returns a PluginMetadata struct. The first field, Name,
// determines the name of the plugin which should generally be without spaces.
// If there are spaces in the name a user will need to properly quote the name
// during uninstall otherwise the name will be treated as seperate arguments.
// The second value is a slice of Command structs. Our slice only contains one
// Command Struct, but could contain any number of them. The first field Name
// defines the command `cf basic-plugin-command` once installed into the CLI. The
// second field, HelpText, is used by the core CLI to display help information
// to the user in the core commands `cf help`, `cf`, or `cf -h`.
func (c *CreateServicePush) GetMetadata() plugin.PluginMetadata {
	return plugin.PluginMetadata{
		Name: "Create-Service-Push",
		Version: plugin.VersionType{
			Major: 1,
			Minor: 0,
			Build: 1,
		},
		MinCliVersion: plugin.VersionType{
			Major: 6,
			Minor: 7,
			Build: 0,
		},
		Commands: []plugin.Command{
			{
				Name:     "create-service-push",
				HelpText: "Works in the same manner as cf push, except that it will create services defined in a services-manifest.yml file first before performing a cf push.",

				// UsageDetails is optional
				// It is used to show help of usage of each command
				UsageDetails: plugin.Usage{
					Usage: "create-service-push\n   cf create-service-push",
					Options: map[string]string{
						"--service-manifest <MANIFEST_FILE>": "Specify the fullpath and filename of the services creation manifest.  Defaults to services-manifest.yml.",
						"--no-service-manifest":              "Specifies that there is no service creation manifest",
						"--no-push":                          "Create the services but do not push the application",
					},
				},
			},
		},
	}
}

// Unlike most Go programs, the `Main()` function will not be used to run all of the
// commands provided in your plugin. Main will be used to initialize the plugin
// process, as well as any dependencies you might require for your
// plugin.
func main() {
	// Any initialization for your plugin can be handled here
	//
	// Note: to run the plugin.Start method, we pass in a pointer to the struct
	// implementing the interface defined at "code.cloudfoundry.org/cli/plugin/plugin.go"
	//
	// Note: The plugin's main() method is invoked at install time to collect
	// metadata. The plugin will exit 0 and the Run([]string) method will not be
	// invoked.
	plugin.Start(new(CreateServicePush))
	// Plugin code should be written in the Run([]string) method,
	// ensuring the plugin environment is bootstrapped.
}
