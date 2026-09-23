package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"os"
	"slices"
	"strings"

	"github.com/alecthomas/kong"

	"github.com/monolithiclab/gomddoc/internal/assets"
	"github.com/monolithiclab/gomddoc/internal/config"
	"github.com/monolithiclab/gomddoc/internal/diag"
	"github.com/monolithiclab/gomddoc/internal/doctor"
	"github.com/monolithiclab/gomddoc/internal/provider"
)

// DoctorCmd checks a site's configuration and content.
type DoctorCmd struct {
	Dir           string `arg:"" optional:"" default:"." env:"GOMDDOC_SERVER_DIR" help:"Markdown directory or Git URL to check."`
	JSON          bool   `name:"json" help:"Print the report as JSON (the same document as the MCP tool gomddoc_doctor)."`
	Strict        bool   `name:"strict" help:"Exit non-zero on warnings as well as errors (for CI)."`
	Verbose       bool   `name:"verbose" short:"v" help:"Include info findings (missing descriptions, unused theme vars)."`
	GitSSHKey     string `name:"git-key-file" default:"" env:"GOMDDOC_SERVER_GIT_SSH_KEY" help:"Path to SSH private key file for Git authentication."`
	GitStorageDir string `name:"git-storage-dir" default:"" env:"GOMDDOC_SERVER_GIT_STORAGE_DIR" help:"Directory for disk-based Git clone storage (default: in-memory)."`

	out     io.Writer       // nil: os.Stdout
	environ func() []string // nil: os.Environ
}

// exitCodeError ends a command with a status and nothing more to say: the
// command already printed its result. main exits with it without logging.
type exitCodeError int

func (e exitCodeError) Error() string { return fmt.Sprintf("exit status %d", int(e)) }

// installScriptEnvs are the variables scripts/install.sh reads. The quickstart
// documents them, so doctor must not call them unknown; a test holds this list
// to the script.
var installScriptEnvs = []string{"GOMDDOC_INSTALL_DIR", "GOMDDOC_REQUIRE_COSIGN", "GOMDDOC_VERSION"}

// Run checks the site, prints the report and sets the exit status: 1 on any
// error, or any warning with --strict.
func (d *DoctorCmd) Run(app *kong.Application) error {
	environ := d.environ
	if environ == nil {
		environ = os.Environ
	}
	report := runDoctor(context.Background(), app, d.Dir, buildGitConfig(d.GitSSHKey, d.GitStorageDir, d.Dir), environ(), d.Verbose)

	w := d.out
	if w == nil {
		w = os.Stdout
	}
	if d.JSON {
		data, err := json.MarshalIndent(report, "", "  ")
		if err != nil {
			return err
		}
		if _, err := fmt.Fprintf(w, "%s\n", data); err != nil {
			return err
		}
	} else if err := writeDoctorReport(w, report); err != nil {
		return err
	}

	if report.Summary.Errors > 0 || (d.Strict && report.Summary.Warnings > 0) {
		return exitCodeError(1)
	}
	return nil
}

// runDoctor loads dir fresh — config, provider, pipelines — and checks it.
func runDoctor(ctx context.Context, app *kong.Application, dir string, gitCfg provider.GitProviderConfig, environ []string, verbose bool) doctor.Report {
	ins := config.Inspect(dir)
	cfg := ins.Config
	prov, err := provider.NewProvider(cfg.Server.Dir, cfg.Site.DefaultIndex, cfg.Site.DirIndex, cfg.Site.Exclude, gitCfg)
	if err != nil {
		in := doctorInput(app, dir, ins, environ)
		in.Unreachable = err
		return doctor.Run(ctx, in, doctor.Options{Verbose: verbose})
	}
	defer func() { _ = prov.Close() }()
	return runDoctorWith(ctx, app, dir, ins, prov, environ, verbose)
}

// runDoctorWith checks dir through an open provider. gomddoc_doctor calls it
// with the MCP server's provider: a filesystem provider reads the disk live,
// and a Git provider's clone is reused rather than re-cloned on every call.
// The configuration and every index are rebuilt each time.
func runDoctorWith(ctx context.Context, app *kong.Application, dir string, ins config.Inspection, prov provider.Provider, environ []string, verbose bool) doctor.Report {
	in := doctorInput(app, dir, ins, environ)
	lp, err := setupLanguagePipelines(ins.Config, prov, PipelineOptions{EnableMetadata: true, ReportOnly: true})
	if err != nil {
		in.Producer = []diag.Finding{diag.New("target.read-error", "", 0, "", "could not build the site: "+err.Error(),
			"fix the problems above, then run doctor again")}
		return doctor.Run(ctx, in, doctor.Options{Verbose: verbose})
	}
	in.Producer = lp.Findings
	in.Assets = assets.BuildFS(lp.Default.ContentRoot, embeddedAssets)
	in.Pipelines = []doctor.PipelineView{{Root: lp.Default.ContentRoot, Exclude: lp.Default.Exclude}}
	for _, lang := range slices.Sorted(maps.Keys(lp.ByLang)) {
		p := lp.ByLang[lang]
		in.Pipelines = append(in.Pipelines, doctor.PipelineView{Lang: lang, Root: p.ContentRoot, Exclude: p.Exclude})
	}
	return doctor.Run(ctx, in, doctor.Options{Verbose: verbose})
}

func doctorInput(app *kong.Application, dir string, ins config.Inspection, environ []string) doctor.Input {
	in := doctor.Input{Target: dir, Inspection: ins, Environ: environ, KnownEnvs: knownEnvVars(app)}
	if config.IsGitURL(dir) {
		in.Note = "Git source: checked the provider's clone of the repository"
	}
	return in
}

// knownEnvVars is every variable gomddoc reads: config settings (a map
// setting contributes its prefix, ending in "_"), Kong flag and argument
// envs, and the install script's.
func knownEnvVars(app *kong.Application) []string {
	known := slices.Clone(installScriptEnvs)
	for _, s := range config.Schema() {
		if p, ok := strings.CutSuffix(s.Env, "<KEY>"); ok {
			known = append(known, p)
		} else if s.Env != "" {
			known = append(known, s.Env)
		}
	}
	for _, n := range app.Children {
		for _, f := range n.Flags {
			known = append(known, f.Envs...)
		}
		for _, p := range n.Positional {
			known = append(known, p.Tag.Envs...)
		}
	}
	slices.Sort(known)
	return slices.Compact(known)
}

// writeDoctorReport prints one finding per line with its fix beneath, then
// the summary.
func writeDoctorReport(w io.Writer, r doctor.Report) error {
	var b strings.Builder
	if r.Note != "" {
		fmt.Fprintf(&b, "note: %s\n", r.Note)
	}
	if r.Assumed != "" {
		fmt.Fprintf(&b, "note: %s\n", r.Assumed)
	}
	for _, f := range r.Findings {
		fmt.Fprintf(&b, "%-8s %s  %s  %s\n", f.Severity, location(f), f.Code, f.Message)
		if f.Fix != "" {
			fmt.Fprintf(&b, "%-8s fix: %s\n", "", f.Fix)
		}
	}
	s := r.Summary
	switch {
	case s.Errors+s.Warnings+s.Info == 0:
		b.WriteString("No problems found.\n")
	case s.InfoHidden:
		fmt.Fprintf(&b, "%s, %s (%d info hidden, use -v)\n", plural(s.Errors, "error"), plural(s.Warnings, "warning"), s.Info)
	default:
		fmt.Fprintf(&b, "%s, %s, %d info\n", plural(s.Errors, "error"), plural(s.Warnings, "warning"), s.Info)
	}
	_, err := io.WriteString(w, b.String())
	return err
}

// location is file:line, the file alone, or the key (an env var) when the
// finding has no file.
func location(f diag.Finding) string {
	switch {
	case f.File != "" && f.Line > 0:
		return fmt.Sprintf("%s:%d", f.File, f.Line)
	case f.File != "":
		return f.File
	case f.Key != "":
		return f.Key
	default:
		return "-"
	}
}

func plural(n int, word string) string {
	if n == 1 {
		return "1 " + word
	}
	return fmt.Sprintf("%d %ss", n, word)
}
