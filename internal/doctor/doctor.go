// Package doctor analyses a site and reports every configuration problem and
// the cheap-to-detect content problems, each as a diag.Finding.
//
// It re-implements nothing a runtime producer already detects: config
// loading, normalization and validation (config.Inspect), clean-URL collisions
// and redirect problems (the pipelines' Findings) arrive in Input, found by the
// same code serve runs. doctor adds only the checks with no runtime
// counterpart — unknown config keys and env vars, theme toggles the theme never
// reads, exclude patterns that match nothing, frontmatter types — merges,
// deduplicates and sorts. Run never fails: an unreachable target is itself a
// finding, so callers always get a well-formed Report.
package doctor

import (
	"context"
	"io/fs"

	"github.com/monolithiclab/gomddoc/internal/config"
	"github.com/monolithiclab/gomddoc/internal/diag"
)

// PipelineView is one content pipeline as doctor walks it: the default one
// (Lang "") or a language directory's, with the exclude list its indexes used.
type PipelineView struct {
	Lang    string
	Root    fs.FS
	Exclude []string
}

// Input is everything Run needs; cmd assembles it.
type Input struct {
	Target      string
	Unreachable error // non-nil: the target could not be opened; only that is reported
	Inspection  config.Inspection
	Pipelines   []PipelineView
	Producer    []diag.Finding // the pipelines' findings (LanguagePipeline.Findings)
	Assets      fs.FS          // asset FS as the renderer sees it, for the theme scan
	Environ     []string       // os.Environ()
	KnownEnvs   []string       // exact names, plus prefixes ending in "_" for map settings
	Note        string         // e.g. that a Git source's snapshot was checked
}

// Options tunes a run.
type Options struct {
	Verbose bool // include Info findings
}

// Summary counts every finding, hidden ones included.
type Summary struct {
	Errors     int  `json:"errors"`
	Warnings   int  `json:"warnings"`
	Info       int  `json:"info"`
	InfoHidden bool `json:"info_hidden"`
}

// Report is a run's result.
type Report struct {
	Target   string         `json:"target"`
	Summary  Summary        `json:"summary"`
	Assumed  string         `json:"assumed,omitempty"`
	Note     string         `json:"note,omitempty"`
	Findings []diag.Finding `json:"findings"`
}

// Run checks in and reports.
func Run(ctx context.Context, in Input, opts Options) Report {
	r := Report{Target: in.Target, Note: in.Note, Findings: []diag.Finding{}}
	var all []diag.Finding
	if in.Unreachable != nil {
		all = []diag.Finding{diag.New("target.unreachable", "", 0, "", in.Unreachable.Error(),
			"check the path or URL, and credentials for a private repository")}
	} else {
		r.Assumed = in.Inspection.Assumed
		all = append(all, in.Inspection.Findings...)
		all = append(all, unknownKeys(in.Inspection.Node)...)
		all = append(all, unknownEnv(in.Environ, in.KnownEnvs)...)
		all = append(all, in.Producer...)
		all = append(all, extraChecks(ctx, in)...)
	}

	all = diag.Dedupe(all)
	diag.Sort(all)
	for _, f := range all {
		switch f.Severity {
		case diag.Error:
			r.Summary.Errors++
		case diag.Warning:
			r.Summary.Warnings++
		default:
			r.Summary.Info++
			if !opts.Verbose {
				r.Summary.InfoHidden = true
				continue
			}
		}
		r.Findings = append(r.Findings, f)
	}
	return r
}

// extraChecks runs the theme, exclude and content checks.
func extraChecks(ctx context.Context, in Input) []diag.Finding {
	scan := scanTheme(in.Inspection, in.Assets)
	findings := themeChecks(in.Inspection, scan)
	for _, p := range in.Pipelines {
		if p.Lang == "" {
			findings = append(findings, excludeMatchesNothing(in.Inspection, p.Root)...)
		}
		findings = append(findings, contentChecks(ctx, p, scan)...)
	}
	return findings
}
