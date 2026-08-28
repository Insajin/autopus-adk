package omp

import "os"

func (a *Adapter) fileModeResolverAt(workspace *ompRootedWorkspace) func(string) os.FileMode {
	return func(path string) os.FileMode {
		if !isOwnerOnlyOMPModelPath(path) {
			return 0o644
		}
		if path != configFile {
			return 0o600
		}
		if info, err := workspace.lstat(path); err == nil {
			return info.Mode().Perm()
		}
		return ompConfigFileMode
	}
}
