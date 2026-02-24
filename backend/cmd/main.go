package main

import (
	"io"
	_ "net/http/pprof"
	"os"
	"os/signal"
	"syscall"

	adj_module "github.com/HyperloopUPV-H8/h9-backend/internal/adj"
	"github.com/HyperloopUPV-H8/h9-backend/internal/config"
	"github.com/HyperloopUPV-H8/h9-backend/internal/flags"
	"github.com/HyperloopUPV-H8/h9-backend/internal/pod_data"
	"github.com/HyperloopUPV-H8/h9-backend/internal/update_factory"
	vehicle_models "github.com/HyperloopUPV-H8/h9-backend/internal/vehicle/models"
	"github.com/HyperloopUPV-H8/h9-backend/pkg/transport"
	"github.com/HyperloopUPV-H8/h9-backend/pkg/websocket"
	trace "github.com/rs/zerolog/log"
)

const (
	BACKEND          = "backend"
	BLCU             = "BLCU"
	TcpClient        = "TCP_CLIENT"
	TcpServer        = "TCP_SERVER"
	UDP              = "UDP"
	SNTP             = "SNTP"
	AddStateOrder    = "add_state_order"
	RemoveStateOrder = "remove_state_order"
)

func main() {
	// Parse command line flags
	flags.Init()
	handleVersionFlag()

	// Configure trace
	traceCloser := initTrace(flags.TraceLevel, flags.TraceFile)
	if traceCloser != nil {
		defer traceCloser.Close()
	}

	// Set use to all available CPUs and setup CPU profiling if enabled
	cleanup := setupRuntimeCPU()
	defer cleanup()

	// <--- config --->
	config, err := config.GetConfig(flags.ConfigFile)
	if err != nil {
		trace.Fatal().Err(err).Msg("error unmarshaling toml file")
	}

	// <--- ADJ --->
	adj, err := adj_module.NewADJ(config.Adj.Branch)
	if err != nil {
		trace.Fatal().Err(err).Msg("setting up ADJ")
	}

	// <--- pod data --->
	podData, err := pod_data.NewPodData(adj.Boards, adj.Info.Units)
	if err != nil {
		trace.Fatal().Err(err).Msg("creating podData")
	}

	// <--- vehicle orders --->
	vehicleOrders, err := vehicle_models.NewVehicleOrders(podData.Boards)
	if err != nil {
		trace.Fatal().Err(err).Msg("creating vehicleOrders")
	}

	// <-- lookup tables -->
	idToBoard, ipToBoardID, boardToPackets := createLookupTables(podData, adj)

	// <--- update factory --->
	updateFactory := update_factory.NewFactory(boardToPackets)

	// <--- logger --->
	loggerHandler, subloggers := setUpLogger(config)

	// <-- connections & upgrader -->
	connections := make(chan *websocket.Client)
	upgrader := websocket.NewUpgrader(connections, trace.Logger)

	// <--- broker --->
	broker, cleanup := configureBroker(subloggers, loggerHandler, idToBoard, connections)
	defer cleanup()

	// <--- transport --->
	transp := transport.NewTransport(trace.Logger)
	transp.SetpropagateFault(config.Transport.PropagateFault)

	// <--- vehicle --->
	err = configureVehicle(
		broker,
		loggerHandler,
		updateFactory,
		ipToBoardID,
		idToBoard,
		transp,
		adj,
		config,
	)
	if err != nil {
		trace.Err(err).Msg("configuring vehicle")
	}

	// <--- transport --->
	configureTransport(
		adj,
		podData,
		transp,
		config,
	)

	// <--- http server --->
	configureHTTPServer(
		adj,
		podData,
		vehicleOrders,
		upgrader,
		config,
	)

	// <--- SNTP --->
	terminate := configureSNTP(adj)
	if terminate {
		os.Exit(1)
	}

	// Start logger
	if flags.EnableLooger {
		err = loggerHandler.Start()
		if err != nil {
			trace.Fatal().Err(err).Msg("starting logger")
		}
	}

	// Open browser tabs
	openBrowserTabs(config)

	// Wait for interrupt signal to gracefully shutdown the backend
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt, syscall.SIGTERM) // syscall.SIGTERM is used to make it work with pnpm interrupt
	defer signal.Stop(interrupt)

	// Windows graceful shutdown
	go func() {
		// io.Copy will block until Stdin is closed (EOF).
		// On Windows, when Electron calls backendProcess.stdin.end() or exits,
		// this pipe will close and trigger a graceful shutdown.
		_, _ = io.Copy(io.Discard, os.Stdin)
		interrupt <- syscall.SIGTERM
	}()

	<-interrupt
	trace.Info().Msg("shutting down backend")
}

// <-- Hall of Fame -->
// H09 -- Zürich    -- PM Juan Martínez, Marc Sanchis -- Winners
// H10 -- Groningen -- PM Marc Sanchis, Joan Física   -- 3rd Place
