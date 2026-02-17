package webserver

import (
	"fmt"
	"log"
	"net/http"
	"strconv"
	"sync"

	cnt "github.com/R3DPanda1/LWN-Sim-Plus/controllers"
	"github.com/R3DPanda1/LWN-Sim-Plus/models"
	"github.com/R3DPanda1/LWN-Sim-Plus/simulator/components/codec"
	"github.com/R3DPanda1/LWN-Sim-Plus/simulator/components/integration"
	"github.com/R3DPanda1/LWN-Sim-Plus/simulator/components/template"
	dev "github.com/R3DPanda1/LWN-Sim-Plus/simulator/components/device"
	rp "github.com/R3DPanda1/LWN-Sim-Plus/simulator/components/device/regional_parameters"
	mrp "github.com/R3DPanda1/LWN-Sim-Plus/simulator/components/device/regional_parameters/models_rp"
	gw "github.com/R3DPanda1/LWN-Sim-Plus/simulator/components/gateway"
	"github.com/R3DPanda1/LWN-Sim-Plus/simulator/events"
	"github.com/R3DPanda1/LWN-Sim-Plus/socket"
	_ "github.com/R3DPanda1/LWN-Sim-Plus/webserver/statik"
	"github.com/brocaar/lorawan"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	socketio "github.com/googollee/go-socket.io"
	"github.com/rakyll/statik/fs"
)

// connSubscriptions holds the active event stream unsubscribe functions for a single socket connection.
type connSubscriptions struct {
	mu    sync.Mutex
	funcs []func()
}

// WebServer represents a web server configuration including address, port, router setup, and server socket.
type WebServer struct {
	Address      string           // Address of the web server
	Port         int              // Port of the web server
	Router       *gin.Engine      // Router of the web server
	ServerSocket *socketio.Server // ServerSocket of the web server
}

// Global variables
var (
	simulatorController cnt.SimulatorController // simulatorController is an instance of the cSimulatorController interface for managing simulator operations.
	configuration       *models.ServerConfig    // configuration is a pointer to models.ServerConfig struct which holds the server's configuration settings.
	// socketSubscriptions tracks active event stream unsubscribe functions per socket connection.
	socketSubscriptions sync.Map // map[string][]func() keyed by socket ID
)

// NewWebServer creates a new web server instance with the given configuration and simulator controller.
func NewWebServer(config *models.ServerConfig, controller cnt.SimulatorController) *WebServer {
	// Storing the configuration and controller instances in the global variables.
	configuration = config
	simulatorController = controller
	serverSocket := newServerSocket()
	// Start the server socket in a separate goroutine due to its blocking nature.
	// If an error occurs, log it and terminate the program.
	go func() {
		err := serverSocket.Serve()
		if err != nil {
			log.Fatal(fmt.Errorf("[WS] [ERROR] [SERVERSOCKET]: %w", err))
		}
	}()
	// Initialize the Gin router and setting up the CORS configuration.
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	configCors := cors.DefaultConfig()
	configCors.AllowAllOrigins = true
	configCors.AllowHeaders = []string{"Origin", "Access-Control-Allow-Origin",
		"Access-Control-Allow-Headers", "Content-type"}
	configCors.AllowMethods = []string{"GET", "POST", "DELETE", "OPTIONS"}
	configCors.AllowCredentials = true
	router.Use(cors.New(configCors))
	// Recovery middleware recovers from any panics and writes a 500 if there was one.
	router.Use(gin.Recovery())
	// Create a new WebServer instance with the given configuration and router.
	ws := WebServer{
		Address:      configuration.Address,
		Port:         configuration.Port,
		Router:       router,
		ServerSocket: serverSocket,
	}
	// Serve the static files using the statik file system.
	staticFS, err := fs.New()
	if err != nil {
		log.Fatal(err)
	}
	staticGroup := router.Group("/dashboard")
	staticGroup.StaticFS("/", staticFS)
	// Set up the API routes.
	apiRoutes := router.Group("/api")
	{
		apiRoutes.GET("/start", startSimulator)        // Start the simulator
		apiRoutes.GET("/stop", stopSimulator)          // Stop the simulator
		apiRoutes.GET("/status", simulatorStatus)      // Get the simulator status (running or stopped)
		apiRoutes.GET("/bridge", getRemoteAddress)     // Get the remote address of the bridge
		apiRoutes.GET("/gateways", getGateways)        // Get the list of gateways
		apiRoutes.GET("/devices", getDevices)          // Get the list of devices
		apiRoutes.POST("/add-device", addDevice)       // Add a new device
		apiRoutes.POST("/up-device", updateDevice)     // Update a device
		apiRoutes.POST("/del-device", deleteDevice)    // Delete a device
		apiRoutes.POST("/del-gateway", deleteGateway)  // Delete a gateway
		apiRoutes.POST("/add-gateway", addGateway)     // Add a new gateway
		apiRoutes.POST("/up-gateway", updateGateway)   // Update a gateway
		apiRoutes.POST("/bridge/save", saveInfoBridge) // Save the remote address of the bridge
		apiRoutes.GET("/codecs", getCodecs)                  // Get all available codecs
		apiRoutes.GET("/codec/:id", getCodec)                // Get a specific codec by ID
		apiRoutes.GET("/codec/:id/usage", getCodecUsage)     // Check which devices use this codec
		apiRoutes.POST("/add-codec", addCodec)               // Add a custom codec
		apiRoutes.POST("/update-codec", updateCodec)         // Update an existing codec
		apiRoutes.POST("/delete-codec", deleteCodec)         // Delete a codec by ID

		// Integration management endpoints
		apiRoutes.GET("/integrations", getIntegrations)                    // Get all integrations
		apiRoutes.GET("/integration/:id", getIntegration)                  // Get a specific integration
		apiRoutes.POST("/add-integration", addIntegration)                 // Add a new integration
		apiRoutes.POST("/update-integration", updateIntegration)           // Update an integration
		apiRoutes.POST("/delete-integration", deleteIntegration)           // Delete an integration
		apiRoutes.POST("/integration/:id/test", testIntegrationConnection) // Test connection to an integration
		apiRoutes.GET("/integration/:id/device-profiles", getDeviceProfiles) // Get device profiles from ChirpStack

		// Template management endpoints
		apiRoutes.GET("/templates", getTemplates)                                  // Get all templates
		apiRoutes.GET("/template/:id", getTemplate)                                // Get a specific template
		apiRoutes.POST("/add-template", addTemplate)                               // Add a new template
		apiRoutes.POST("/update-template", updateTemplate)                         // Update a template
		apiRoutes.POST("/delete-template", deleteTemplate)                         // Delete a template
		apiRoutes.POST("/create-devices-from-template", createDevicesFromTemplate) // Bulk create devices from template
	}
	// Set up the WebSocket routes.
	router.GET("/socket.io/*any", gin.WrapH(serverSocket))
	router.POST("/socket.io/*any", gin.WrapH(serverSocket))
	// Redirect the root path to the dashboard.
	router.GET("/", func(context *gin.Context) { context.Redirect(http.StatusMovedPermanently, "/dashboard") })
	return &ws
}

// newServerSocket creates a new server socket instance and sets up the socket events.
func newServerSocket() *socketio.Server {
	serverSocket := socketio.NewServer(nil)
	serverSocket.OnConnect("/", func(s socketio.Conn) error {
		log.Println("[WS]: Socket connected")
		s.SetContext("")
		simulatorController.AddWebSocket(&s)
		return nil
	})
	serverSocket.OnDisconnect("/", func(s socketio.Conn, reason string) {
		cleanupSocketSubscriptions(s.ID())
		serverSocket.Remove(s.ID())
		_ = s.Close()
	})
	serverSocket.OnEvent("/", socket.EventToggleStateDevice, func(s socketio.Conn, Id int) {
		simulatorController.ToggleStateDevice(Id)
	})
	serverSocket.OnEvent("/", socket.EventToggleStateGateway, func(s socketio.Conn, Id int) {
		simulatorController.ToggleStateGateway(Id)
	})
	serverSocket.OnEvent("/", socket.EventMacCommand, func(s socketio.Conn, data socket.MacCommand) {

		switch data.CID {
		case "DeviceTimeReq":
			simulatorController.SendMACCommand(lorawan.DeviceTimeReq, data)
		case "LinkCheckReq":
			simulatorController.SendMACCommand(lorawan.LinkCheckReq, data)
		case "PingSlotInfoReq":
			simulatorController.SendMACCommand(lorawan.PingSlotInfoReq, data)
		}

	})
	serverSocket.OnEvent("/", socket.EventChangePayload, func(s socketio.Conn, data socket.NewPayload) (string, bool) {
		return simulatorController.ChangePayload(data)
	})
	serverSocket.OnEvent("/", socket.EventSendUplink, func(s socketio.Conn, data socket.NewPayload) {
		simulatorController.SendUplink(data)
	})
	serverSocket.OnEvent("/", socket.EventGetParameters, func(s socketio.Conn, code int) mrp.Informations {
		return rp.GetInfo(code)
	})
	serverSocket.OnEvent("/", socket.EventChangeLocation, func(s socketio.Conn, info socket.NewLocation) bool {
		return simulatorController.ChangeLocation(info)
	})

	// Event stream subscriptions
	serverSocket.OnEvent("/", socket.EventStreamDeviceEvents, func(s socketio.Conn, req socket.StreamRequest) {
		broker := simulatorController.GetEventBroker()
		if broker == nil {
			return
		}
		topic := events.DeviceTopic(req.DevEUI)
		ch, history, unsub := broker.Subscribe(topic)
		addSocketSubscription(s.ID(), unsub)

		// Send history first
		for _, evt := range history {
			s.Emit(socket.EventDeviceEvent, evt)
		}

		// Forward live events
		go func() {
			for evt := range ch {
				s.Emit(socket.EventDeviceEvent, evt)
			}
		}()
	})
	serverSocket.OnEvent("/", socket.EventStopDeviceEvents, func(s socketio.Conn, req socket.StreamRequest) {
		cleanupSocketSubscriptions(s.ID())
	})
	serverSocket.OnEvent("/", socket.EventStreamGatewayEvents, func(s socketio.Conn, req socket.StreamRequest) {
		broker := simulatorController.GetEventBroker()
		if broker == nil {
			return
		}
		topic := events.GatewayTopic(req.GatewayMAC)
		ch, history, unsub := broker.Subscribe(topic)
		addSocketSubscription(s.ID(), unsub)

		for _, evt := range history {
			s.Emit(socket.EventGatewayEvent, evt)
		}

		go func() {
			for evt := range ch {
				s.Emit(socket.EventGatewayEvent, evt)
			}
		}()
	})
	serverSocket.OnEvent("/", socket.EventStopGatewayEvents, func(s socketio.Conn, req socket.StreamRequest) {
		cleanupSocketSubscriptions(s.ID())
	})

	return serverSocket
}

func addSocketSubscription(socketID string, unsub func()) {
	val, _ := socketSubscriptions.LoadOrStore(socketID, &connSubscriptions{})
	entry := val.(*connSubscriptions)
	entry.mu.Lock()
	entry.funcs = append(entry.funcs, unsub)
	entry.mu.Unlock()
}

func cleanupSocketSubscriptions(socketID string) {
	val, ok := socketSubscriptions.LoadAndDelete(socketID)
	if !ok {
		return
	}
	entry := val.(*connSubscriptions)
	entry.mu.Lock()
	for _, fn := range entry.funcs {
		fn()
	}
	entry.funcs = nil
	entry.mu.Unlock()
}

// Run starts the web server and listens on the given address and port.
func (ws *WebServer) Run() {
	fullAddress := ws.Address + ":" + strconv.Itoa(ws.Port)
	log.Printf("[WS]: Listen [%s]", fullAddress)
	err := ws.Router.Run(fullAddress)
	// If an error occurs, log it and terminate the program.
	if err != nil {
		log.Fatal(fmt.Errorf("[WS] [ERROR]: %w", err))
	}
}

// --- API Handlers ---
// startSimulator starts the simulator
func startSimulator(c *gin.Context) {
	c.JSON(http.StatusOK, simulatorController.Run())
}

// stopSimulator stops the simulator
func stopSimulator(c *gin.Context) {
	c.JSON(http.StatusOK, simulatorController.Stop())
}

// simulatorStatus returns the status of the simulator
func simulatorStatus(c *gin.Context) {
	c.JSON(http.StatusOK, simulatorController.Status())
}

// saveInfoBridge saves the remote address of the bridge
func saveInfoBridge(c *gin.Context) {
	var ns models.AddressIP
	err := c.BindJSON(&ns)
	// If an error occurs, return a bad request status.
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "Invalid request"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": simulatorController.SaveBridgeAddress(ns)})
}

// getRemoteAddress returns the remote address of the bridge
func getRemoteAddress(c *gin.Context) {
	c.JSON(http.StatusOK, simulatorController.GetBridgeAddress())
}

// getGateways returns the list of gateways
func getGateways(c *gin.Context) {
	gws := simulatorController.GetGateways()
	c.JSON(http.StatusOK, gws)
}

// addGateway adds a new gateway
func addGateway(c *gin.Context) {
	var g gw.Gateway
	err := c.BindJSON(&g)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "Invalid request"})
		return
	}
	code, id, err := simulatorController.AddGateway(&g)
	errString := fmt.Sprintf("%v", err)
	c.JSON(http.StatusOK, gin.H{"status": errString, "code": code, "id": id})
}

// updateGateway updates a gateway
func updateGateway(c *gin.Context) {
	var g gw.Gateway
	err := c.BindJSON(&g)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "Invalid request"})
		return
	}
	code, err := simulatorController.UpdateGateway(&g)
	errString := fmt.Sprintf("%v", err)
	c.JSON(http.StatusOK, gin.H{"status": errString, "code": code})
}

// deleteGateway deletes a gateway
func deleteGateway(c *gin.Context) {
	Identifier := struct {
		Id int `json:"id"`
	}{}
	err := c.BindJSON(&Identifier)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "Invalid request"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": simulatorController.DeleteGateway(Identifier.Id)})
}

// getDevices returns the list of devices
func getDevices(c *gin.Context) {
	c.JSON(http.StatusOK, simulatorController.GetDevices())
}

// addDevice adds a new device
func addDevice(c *gin.Context) {
	var device dev.Device
	err := c.BindJSON(&device)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "Invalid request"})
		return
	}
	code, id, err := simulatorController.AddDevice(&device)
	errString := fmt.Sprintf("%v", err)
	c.JSON(http.StatusOK, gin.H{"status": errString, "code": code, "id": id})
}

// updateDevice updates a device
func updateDevice(c *gin.Context) {
	var device dev.Device
	err := c.BindJSON(&device)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "Invalid request"})
		return
	}
	code, err := simulatorController.UpdateDevice(&device)
	errString := fmt.Sprintf("%v", err)
	c.JSON(http.StatusOK, gin.H{"status": errString, "code": code})
}

// deleteDevice deletes a device
func deleteDevice(c *gin.Context) {
	Identifier := struct {
		Id int `json:"id"`
	}{}
	err := c.BindJSON(&Identifier)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "Invalid request"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": simulatorController.DeleteDevice(Identifier.Id)})
}

// getCodecs returns all available codecs
func getCodecs(c *gin.Context) {
	codecs := simulatorController.GetCodecs()
	c.JSON(http.StatusOK, gin.H{"codecs": codecs})
}

// getCodec returns a specific codec by ID
func getCodec(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "Invalid codec ID", "error": err.Error()})
		return
	}
	codec, err := simulatorController.GetCodec(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"status": "Codec not found", "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"codec": codec})
}

// addCodec adds a custom codec
func addCodec(c *gin.Context) {
	var codecData struct {
		Name   string `json:"name"`
		Script string `json:"script"`
	}

	if err := c.BindJSON(&codecData); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "Invalid JSON", "error": err.Error()})
		return
	}

	// Create new codec
	newCodec := codec.NewCodec(codecData.Name, codecData.Script)

	// Add to manager
	if err := simulatorController.AddCodec(newCodec); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "Failed to add codec", "error": err.Error()})
		return
	}

	// Emit WebSocket event
	simulatorController.EmitCodecEvent(socket.EventCodecAdded, newCodec.Metadata())

	c.JSON(http.StatusOK, gin.H{"status": "Codec added successfully", "id": newCodec.ID})
}

// updateCodec updates an existing codec
func updateCodec(c *gin.Context) {
	var codecData struct {
		ID     int    `json:"id"`
		Name   string `json:"name"`
		Script string `json:"script"`
	}

	if err := c.BindJSON(&codecData); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "Invalid JSON", "error": err.Error()})
		return
	}

	// Validate required fields
	if codecData.ID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"status": "Invalid request", "error": "ID is required"})
		return
	}

	// Update codec
	if err := simulatorController.UpdateCodec(codecData.ID, codecData.Name, codecData.Script); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "Failed to update codec", "error": err.Error()})
		return
	}

	// Get updated codec for metadata
	updatedCodec, err := simulatorController.GetCodec(codecData.ID)
	if err != nil {
		// Still return success but without metadata
		c.JSON(http.StatusOK, gin.H{"status": "Codec updated successfully", "id": codecData.ID})
		return
	}

	// Emit WebSocket event
	simulatorController.EmitCodecEvent(socket.EventCodecUpdated, updatedCodec.Metadata())

	c.JSON(http.StatusOK, gin.H{"status": "Codec updated successfully", "id": codecData.ID})
}

// deleteCodec deletes a codec by ID
func deleteCodec(c *gin.Context) {
	var reqData struct {
		ID int `json:"id"`
	}

	if err := c.BindJSON(&reqData); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "Invalid JSON", "error": err.Error()})
		return
	}

	if err := simulatorController.DeleteCodec(reqData.ID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "Failed to delete codec", "error": err.Error()})
		return
	}

	// Emit WebSocket event
	simulatorController.EmitCodecEvent(socket.EventCodecDeleted, gin.H{"id": reqData.ID})

	c.JSON(http.StatusOK, gin.H{"status": "Codec deleted successfully"})
}

// getCodecUsage returns which devices are using a specific codec
func getCodecUsage(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid codec ID"})
		return
	}
	devices := simulatorController.GetDevicesUsingCodec(id)
	c.JSON(http.StatusOK, gin.H{"codecId": id, "devices": devices, "count": len(devices)})
}

// ==================== Integration Handlers ====================

// getIntegrations returns all integrations
func getIntegrations(c *gin.Context) {
	integrations := simulatorController.GetIntegrations()
	c.JSON(http.StatusOK, gin.H{"integrations": integrations})
}

// getIntegration returns a specific integration by ID
func getIntegration(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "Invalid integration ID", "error": err.Error()})
		return
	}
	integ, err := simulatorController.GetIntegration(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"status": "Integration not found", "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"integration": integ})
}

// addIntegration adds a new integration
func addIntegration(c *gin.Context) {
	var data struct {
		Name          string `json:"name"`
		Type          string `json:"type"`
		URL           string `json:"url"`
		APIKey        string `json:"apiKey"`
		TenantID      string `json:"tenantId"`
		ApplicationID string `json:"applicationId"`
	}

	if err := c.BindJSON(&data); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	intType := integration.IntegrationType(data.Type)
	if intType == "" {
		intType = integration.IntegrationTypeChirpStack
	}

	id, err := simulatorController.AddIntegration(data.Name, intType, data.URL, data.APIKey, data.TenantID, data.ApplicationID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	simulatorController.EmitIntegrationEvent(socket.EventIntegrationAdded, gin.H{"id": id, "name": data.Name})
	c.JSON(http.StatusOK, gin.H{"id": id})
}

// updateIntegration updates an existing integration
func updateIntegration(c *gin.Context) {
	var data struct {
		ID            *int   `json:"id"` // Pointer to distinguish between "not provided" and "0"
		Name          string `json:"name"`
		URL           string `json:"url"`
		APIKey        string `json:"apiKey"`
		TenantID      string `json:"tenantId"`
		ApplicationID string `json:"applicationId"`
		Enabled       bool   `json:"enabled"`
	}

	if err := c.BindJSON(&data); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if data.ID == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ID is required"})
		return
	}

	if err := simulatorController.UpdateIntegration(*data.ID, data.Name, data.URL, data.APIKey, data.TenantID, data.ApplicationID, data.Enabled); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	simulatorController.EmitIntegrationEvent(socket.EventIntegrationUpdated, gin.H{"id": *data.ID, "name": data.Name})
	c.JSON(http.StatusOK, gin.H{"success": true})
}

// deleteIntegration removes an integration
func deleteIntegration(c *gin.Context) {
	var data struct {
		ID int `json:"id"`
	}

	if err := c.BindJSON(&data); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := simulatorController.DeleteIntegration(data.ID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	simulatorController.EmitIntegrationEvent(socket.EventIntegrationDeleted, gin.H{"id": data.ID})
	c.JSON(http.StatusOK, gin.H{"success": true})
}

// testIntegrationConnection tests connection to an integration
func testIntegrationConnection(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid integration ID"})
		return
	}

	if err := simulatorController.TestIntegrationConnection(id); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}

// getDeviceProfiles returns device profiles for an integration
func getDeviceProfiles(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid integration ID"})
		return
	}

	profiles, err := simulatorController.GetDeviceProfiles(id)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"deviceProfiles": profiles})
}

// ==================== Template Handlers ====================

// getTemplates returns all templates
func getTemplates(c *gin.Context) {
	templates := simulatorController.GetTemplates()
	c.JSON(http.StatusOK, gin.H{"templates": templates})
}

// getTemplate returns a specific template by ID
func getTemplate(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid template ID"})
		return
	}
	tmpl, err := simulatorController.GetTemplate(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"template": tmpl})
}

// addTemplate adds a new template
func addTemplate(c *gin.Context) {
	var tmpl template.DeviceTemplate

	if err := c.BindJSON(&tmpl); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	id, err := simulatorController.AddTemplate(&tmpl)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"id": id})
}

// updateTemplate updates an existing template
func updateTemplate(c *gin.Context) {
	var tmpl template.DeviceTemplate

	if err := c.BindJSON(&tmpl); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if tmpl.ID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ID is required"})
		return
	}

	if err := simulatorController.UpdateTemplate(&tmpl); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}

// deleteTemplate removes a template
func deleteTemplate(c *gin.Context) {
	var data struct {
		ID int `json:"id"`
	}

	if err := c.BindJSON(&data); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := simulatorController.DeleteTemplate(data.ID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}

// BulkDeviceRequest represents the request for bulk device creation
type BulkDeviceRequest struct {
	TemplateID   int     `json:"templateId"`
	Count        int     `json:"count"`
	NamePrefix   string  `json:"namePrefix"`
	BaseLat      float64 `json:"baseLat"`
	BaseLng      float64 `json:"baseLng"`
	BaseAlt      int32   `json:"baseAlt"`
	SpreadMeters float64 `json:"spreadMeters"`
}

// createDevicesFromTemplate creates multiple devices from a template
func createDevicesFromTemplate(c *gin.Context) {
	var req BulkDeviceRequest

	if err := c.BindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Validate request
	if req.TemplateID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "templateId is required"})
		return
	}
	if req.Count < 1 || req.Count > 1000 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "count must be between 1 and 1000"})
		return
	}
	if req.NamePrefix == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "namePrefix is required"})
		return
	}
	if req.SpreadMeters <= 0 {
		req.SpreadMeters = 100 // Default 100m spread
	}

	createdIDs, err := simulatorController.CreateDevicesFromTemplate(req.TemplateID, req.Count, req.NamePrefix, req.BaseLat, req.BaseLng, req.BaseAlt, req.SpreadMeters)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"created": len(createdIDs), "deviceIds": createdIDs})
}
