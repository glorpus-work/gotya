//go:generate mockgen -package artifact -destination=./hooks_mock.go . HookExecutor
package artifact

import (
	"fmt"
	"os"

	"github.com/d5/tengo/v2"
	"github.com/d5/tengo/v2/stdlib"
	"github.com/glorpus-work/gotya/internal/logger"
	"github.com/glorpus-work/gotya/pkg/errors"
)

// HookExecutor manages the execution of Tengo script hooks
type HookExecutor interface {
	ExecuteHook(hookPath string, context *HookContext) error
}

// HookContext provides context information to hook scripts
type HookContext struct {
	ArtifactName    string
	ArtifactVersion string
	Operation       string // "install", "update", "uninstall"
	MetaDir         string
	DataDir         string
	TempMetaDir     string // For pre-install hooks (temp extraction dir)
	FinalMetaDir    string // For pre-install hooks (final install dir)
	FinalDataDir    string // For pre-install hooks (final install dir)
	WasMetaDir      string // For post-uninstall hooks (where meta dir was)
	WasDataDir      string // For post-uninstall hooks (where data dir was)
	OldVersion      string // For updates (previous version)
}

// HookExecutorImpl is the default implementation of HookExecutor
type HookExecutorImpl struct{}

// NewHookExecutor creates a new hook executor instance
func NewHookExecutor() *HookExecutorImpl {
	return &HookExecutorImpl{}
}

// ExecuteHook executes a Tengo script hook with the provided context
func (he *HookExecutorImpl) ExecuteHook(hookPath string, context *HookContext) error {
	if _, err := os.Stat(hookPath); os.IsNotExist(err) {
		return errors.Wrapf(errors.ErrValidation, "hook script %s does not exist", hookPath)
	}

	logger.Debug("Executing hook script", logger.Fields{
		"hook_path": hookPath,
		"operation": context.Operation,
		"artifact":  context.ArtifactName,
		"version":   context.ArtifactVersion,
	})

	// Read the script file
	scriptContent, err := os.ReadFile(hookPath)
	if err != nil {
		return fmt.Errorf("failed to read hook script %s: %w", hookPath, err)
	}

	// Create Tengo script with module map for variables
	moduleMap := stdlib.GetModuleMap(stdlib.AllModuleNames()...)
	he.setupScriptContext(moduleMap, context)

	// Create script with the module map
	script := tengo.NewScript(scriptContent)
	script.SetImports(moduleMap)

	// Execute the script
	if _, err := script.Run(); err != nil {
		return errors.Wrapf(err, "hook script execution failed for %s", hookPath)
	}

	logger.Debug("Hook script executed successfully", logger.Fields{
		"hook_path": hookPath,
		"operation": context.Operation,
		"artifact":  context.ArtifactName,
	})

	return nil
}

// setupScriptContext sets up the Tengo script context variables
func (he *HookExecutorImpl) setupScriptContext(moduleMap *tengo.ModuleMap, context *HookContext) {
	// Set standard context variables
	moduleMap.AddBuiltinModule("context", map[string]tengo.Object{
		"artifact_name":    &tengo.String{Value: context.ArtifactName},
		"artifact_version": &tengo.String{Value: context.ArtifactVersion},
		"operation":        &tengo.String{Value: context.Operation},
	})

	// Set directory paths based on what's available
	dirModule := make(map[string]tengo.Object)
	if context.MetaDir != "" {
		dirModule["meta_dir"] = &tengo.String{Value: context.MetaDir}
	}
	if context.DataDir != "" {
		dirModule["data_dir"] = &tengo.String{Value: context.DataDir}
	}
	if context.TempMetaDir != "" {
		dirModule["temp_meta_dir"] = &tengo.String{Value: context.TempMetaDir}
	}
	if context.FinalMetaDir != "" {
		dirModule["final_meta_dir"] = &tengo.String{Value: context.FinalMetaDir}
	}
	if context.FinalDataDir != "" {
		dirModule["final_data_dir"] = &tengo.String{Value: context.FinalDataDir}
	}
	if context.WasMetaDir != "" {
		dirModule["was_meta_dir"] = &tengo.String{Value: context.WasMetaDir}
	}
	if context.WasDataDir != "" {
		dirModule["was_data_dir"] = &tengo.String{Value: context.WasDataDir}
	}
	if context.OldVersion != "" {
		dirModule["old_version"] = &tengo.String{Value: context.OldVersion}
	}

	if len(dirModule) > 0 {
		moduleMap.AddBuiltinModule("dirs", dirModule)
	}
}
