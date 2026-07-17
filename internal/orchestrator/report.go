package orchestrator

// Report is the data behind `agents status`.
type Report struct {
	RepoRoot        string
	CurrentBranch   string
	DefaultBranch   string
	DetachedHEAD    bool
	MergeInProgress bool
	Session         string
	SessionExists   bool
	Agents          []AgentInfo
}

// StatusReport gathers repository and agent health in one pass.
func (o *Orchestrator) StatusReport() (*Report, error) {
	r := &Report{
		RepoRoot: o.Git.Root(),
		Session:  o.Session,
	}

	current, err := o.Git.CurrentBranch()
	if err != nil {
		return nil, err
	}
	r.CurrentBranch = current
	r.DetachedHEAD = current == ""

	if r.DefaultBranch, err = o.Git.DefaultBranch(); err != nil {
		r.DefaultBranch = ""
	}

	if r.MergeInProgress, err = o.Git.MergeInProgress(); err != nil {
		return nil, err
	}

	r.SessionExists = o.Tmux.HasSession(o.Session)

	if r.Agents, err = o.List(); err != nil {
		return nil, err
	}
	return r, nil
}
