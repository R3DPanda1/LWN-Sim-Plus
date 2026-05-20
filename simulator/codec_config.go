package simulator

import (
	"fmt"

	"github.com/R3DPanda1/LWN-Sim-Plus/models"
	"github.com/R3DPanda1/LWN-Sim-Plus/shared"
	"github.com/R3DPanda1/LWN-Sim-Plus/simulator/components/codec"
	dev "github.com/R3DPanda1/LWN-Sim-Plus/simulator/components/device"
	"github.com/R3DPanda1/LWN-Sim-Plus/simulator/util"
)

// SetCodecConfig initialises the global codec registry from the server config
// before GetInstance() runs. Safe to call once at startup.
func SetCodecConfig(cfg *models.ServerConfig) {
	if dev.Codecs != nil {
		return
	}
	ec := codec.DefaultExecutorConfig()
	if cfg != nil {
		if cfg.MaxCodecVMs != 0 {
			ec.MaxVMs = cfg.MaxCodecVMs
		}
		if cfg.CodecTimeoutMs != 0 {
			ec.TimeoutMs = cfg.CodecTimeoutMs
		}
	}
	initCodecRegistry(ec)
}

// initCodecRegistry creates dev.Codecs and loads the codec library from disk,
// falling back to built-in defaults when no library is found.
func initCodecRegistry(ec *codec.ExecutorConfig) {
	dev.Codecs = codec.NewRegistry(ec)

	pathDir, err := util.GetPath()
	codecLibLoaded := false
	if err == nil {
		codecLibPath := pathDir + "/codecs.json"
		if err := dev.Codecs.Load(codecLibPath); err != nil {
			shared.DebugPrint(fmt.Sprintf("Warning: %v", err))
		} else {
			shared.DebugPrint("Codec library loaded from disk")
			codecLibLoaded = true
		}
	}

	if !codecLibLoaded || dev.Codecs.GetCodecCount() == 0 {
		dev.Codecs.LoadDefaults()
		shared.DebugPrint("Default codecs loaded")
	}

	if err == nil {
		statesPath := pathDir + "/codec_states.json"
		if err := dev.Codecs.LoadStates(statesPath); err != nil {
			shared.DebugPrint(fmt.Sprintf("Warning: %v", err))
		} else {
			shared.DebugPrint("Codec states loaded from disk")
		}
	}

	shared.DebugPrint("Codec manager initialized")
}
