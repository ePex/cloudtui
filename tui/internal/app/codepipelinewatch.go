package app

// IsWatchingPipeline reports whether name currently has an active
// background watch. See view.PipelineWatcher for the implementation.
func (a *App) IsWatchingPipeline(name string) bool {
	return a.pipelineWatcher.IsWatchingPipeline(name)
}

// StartWatchingPipeline begins polling name's stage state in the
// background. See view.PipelineWatcher for the implementation.
func (a *App) StartWatchingPipeline(name string) {
	a.pipelineWatcher.StartWatchingPipeline(name)
}

// StopWatchingPipeline stops name's background watch, if any. See
// view.PipelineWatcher for the implementation.
func (a *App) StopWatchingPipeline(name string) {
	a.pipelineWatcher.StopWatchingPipeline(name)
}
