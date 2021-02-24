package main

import (
	"fmt"
	"io"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"github.com/atif-konasl/eth-research/simple_rpc/api"
	"github.com/atif-konasl/eth-research/simple_rpc/utils"
	"github.com/atif-konasl/eth-research/simple_rpc/rpc"
	cli "gopkg.in/urfave/cli.v1"
)


var (
	orchestratorApi       		api.ExternalAPI
	ipcapiURL = 				"n/a"
	serverListner 				net.Listener

	app         		= cli.NewApp()
	serverStartCommand 	= cli.Command{
		Action:    utils.MigrateFlags(startServer),
		Name:      "start-server",
		Usage:     "Start server",
		ArgsUsage: "",
		Flags: []cli.Flag{
			utils.IPCPathFlag,
			utils.IPCDisabledFlag,
			utils.HTTPEnabledFlag,
			utils.HTTPListenAddrFlag,
			utils.HTTPPortFlag,
			utils.LogLevelFlag,
			utils.ConfigDirFlag,
		},
		Description: `
The start-server command starts the rpc server using http or ipc.`,
	}
	serverStopCommand    = cli.Command{
		Action:    utils.MigrateFlags(stopSever),
		Name:      "stop-server",
		Usage:     "Stop server",
		ArgsUsage: "",
		Flags: []cli.Flag{
			utils.LogLevelFlag,
		},
		Description: `
The stop-server command stop the rpc server using http or ipc.`,
	}
	clientStartCommand = cli.Command{
		Action:    utils.MigrateFlags(client),
		Name:      "client",
		Usage:     "Run client",
		ArgsUsage: "",
		Flags: []cli.Flag{
			utils.LogLevelFlag,
		},
		Description: `
The client command starts the client to communicate with rpc server via http or ipc`,
	}

)

// AppHelpFlagGroups is the application flags, grouped by functionality.
var AppHelpFlagGroups = []utils.FlagGroup{
	{
		Name: "IPC-FLAGS",
		Flags: []cli.Flag{
			utils.IPCDisabledFlag,
			utils.IPCPathFlag,
			utils.ConfigDirFlag,
		},
	},
	{
		Name: "HTTP-FLAGS",
		Flags: []cli.Flag{
			utils.HTTPEnabledFlag,
			utils.HTTPListenAddrFlag,
			utils.HTTPPortFlag,
			utils.ConfigDirFlag,
		},
	},
	{
		Name: "COMMON-FLAGS",
		Flags: []cli.Flag{
			utils.LogLevelFlag,
			utils.ConfigDirFlag,
		},
	},
}


func init() {
	app.Name = "Orchestrator"
	app.Usage = "Orchestrator client is the main consensus client for beaconchain and catalyst chain"
	app.Flags = []cli.Flag{
		utils.HTTPPortFlag,
		utils.HTTPListenAddrFlag,
		utils.HTTPEnabledFlag,
		utils.IPCDisabledFlag,
		utils.IPCPathFlag,
		utils.LogLevelFlag,
		utils.ConfigDirFlag,
	}
	app.Action = startServer

	app.Commands = []cli.Command{
		serverStartCommand,
		serverStopCommand,
		clientStartCommand,
	}

	cli.CommandHelpTemplate = utils.CommandHelpTemplate
	// Override the default app help template
	cli.AppHelpTemplate = utils.OrchestratorAppHelpTemplate

	// Override the default app help printer, but only for the global app help
	originalHelpPrinter := cli.HelpPrinter
	cli.HelpPrinter = func(w io.Writer, tmpl string, data interface{}) {
		if tmpl == utils.OrchestratorAppHelpTemplate {
			// Render out custom usage screen
			originalHelpPrinter(w, tmpl, utils.HelpData{App: data, FlagGroups: AppHelpFlagGroups})
		} else if tmpl == utils.CommandHelpTemplate {
			// Iterate over all command specific flags and categorize them
			categorized := make(map[string][]cli.Flag)
			for _, flag := range data.(cli.Command).Flags {
				if _, ok := categorized[flag.String()]; !ok {
					categorized[utils.FlagCategory(flag, AppHelpFlagGroups)] = append(categorized[utils.FlagCategory(flag, AppHelpFlagGroups)], flag)
				}
			}

			// sort to get a stable ordering
			sorted := make([]utils.FlagGroup, 0, len(categorized))
			for cat, flgs := range categorized {
				sorted = append(sorted, utils.FlagGroup{Name: cat, Flags: flgs})
			}
			sort.Sort(utils.ByCategory(sorted))

			// add sorted array to data and render with default printer
			originalHelpPrinter(w, tmpl, map[string]interface{}{
				"cmd":              data,
				"categorizedFlags": sorted,
			})
		} else {
			originalHelpPrinter(w, tmpl, data)
		}
	}
}

func main() {
	if err := app.Run(os.Args); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func startServer(c *cli.Context) error {
	orchestratorApi := api.NewOrchestratorApi(1, "orchestratorApi")
	rpcAPI := []rpc.API{
		{
			Namespace: "orchestrator",
			Public:    true,
			Service:   orchestratorApi,
			Version:   "1.0",
		},
	}

	configDir := c.GlobalString(utils.ConfigDirFlag.Name)
	if !c.GlobalBool(utils.IPCDisabledFlag.Name) {
		givenPath := c.GlobalString(utils.IPCPathFlag.Name)
		ipcapiURL = ipcEndpoint(filepath.Join(givenPath, "orchestrator.ipc"), configDir)
		listener, _, err := rpc.StartIPCEndpoint(ipcapiURL, rpcAPI)
		if err != nil {
			log.Fatalf("Could not start IPC api: %v", err)
		}
		serverListner = listener
		log.Info("IPC endpoint opened", "url", ipcapiURL)
		defer func() {
			listener.Close()
			log.Info("IPC endpoint closed", "url", ipcapiURL)
		}()
	}

	abortChan := make(chan os.Signal, 1)
	signal.Notify(abortChan, os.Interrupt)

	sig := <-abortChan
	log.Info("Exiting...", "signal", sig)

	return nil
}

//
func stopSever(c *cli.Context) error {
	if serverListner != nil {
		serverListner.Close()
		log.Info("IPC endpoint closed", "url", ipcapiURL)
	}
	return nil
}

//
func client(c *cli.Context) error {

	return nil
}


// ipcEndpoint resolves an IPC endpoint based on a configured value, taking into
// account the set data folders as well as the designated platform we're currently
// running on.
func ipcEndpoint(ipcPath, datadir string) string {
	// On windows we can only use plain top-level pipes
	if runtime.GOOS == "windows" {
		if strings.HasPrefix(ipcPath, `\\.\pipe\`) {
			return ipcPath
		}
		return `\\.\pipe\` + ipcPath
	}
	// Resolve names into the data directory full paths otherwise
	if filepath.Base(ipcPath) == ipcPath {
		if datadir == "" {
			return filepath.Join(os.TempDir(), ipcPath)
		}
		return filepath.Join(datadir, ipcPath)
	}
	return ipcPath
}