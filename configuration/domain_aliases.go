package config

import (
	commandlinetools "aurora/configuration/command_line_tools"
	modelcombinations "aurora/configuration/model_combinations"
	providermodels "aurora/configuration/provider_models"
)

type CLIToolsConfig = commandlinetools.CLIToolsConfig

type ComboDefinition = modelcombinations.ComboDefinition
type CombosConfig = modelcombinations.CombosConfig

type RawProviderModel = providermodels.RawProviderModel

var ProviderModelIDs = providermodels.ProviderModelIDs
var ProviderModelMetadataOverrides = providermodels.ProviderModelMetadataOverrides
