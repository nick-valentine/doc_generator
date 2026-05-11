package generators

import (
	"encoding/json"
	"fmt"
	"html"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"doc_generator/pkg/store"
)

// HTMLGenerator is an output plugin that implements store.Generator.
// It formats parsed files, structs, fields, and methods into a premium multi-page HTML dashboard.
type HTMLGenerator struct{}

// Helper to calculate CRAP index for a symbol based on complexity and optional coverage
func getCRAPScore(sym store.Symbol) int {
	C := sym.Complexity
	if C <= 0 {
		C = 1
	}
	if sym.Coverage != nil {
		cov := *sym.Coverage / 100.0
		crap := float64(C*C)*math.Pow(1.0-cov, 3) + float64(C)
		return int(math.Round(crap))
	}
	return C*C + C
}

// buildSidebar generates a consistent sidebar navigation with relative paths adjusted for depth
func (hg *HTMLGenerator) buildSidebar(source *store.Source, depth int) string {
	relPath := ""
	pagePrefix := "pages/"
	if depth == 1 {
		relPath = "../"
		pagePrefix = ""
	}

	var sb strings.Builder

	// Dashboard & Search Links
	sb.WriteString(fmt.Sprintf(`<div class="nav-section">
        <a href="%[1]sindex.html" class="nav-link" style="font-weight: 600; color: var(--text-primary); display: block; padding: 0.5rem 0.75rem;">📊 Dashboard</a>
        <a href="%[1]spages/search.html" class="nav-link" style="font-weight: 600; color: var(--text-primary); display: block; padding: 0.5rem 0.75rem;">🔍 Search</a>
        <a href="%[1]spages/patterns.html" class="nav-link" style="font-weight: 600; color: var(--text-primary); display: block; padding: 0.5rem 0.75rem;">🧩 Patterns</a>
        <a href="%[1]spages/network.html" class="nav-link" style="font-weight: 600; color: var(--text-primary); display: block; padding: 0.5rem 0.75rem;">🌐 Network Analysis</a>
    </div>`+"\n", relPath))

	// Dynamic Language Collection
	activeLangs := make(map[string]bool)
	for _, f := range source.Files {
		activeLangs[getLanguageFromPath(f.Name)] = true
	}
	var sortedLangs []string
	for l := range activeLangs {
		sortedLangs = append(sortedLangs, l)
	}
	sort.Strings(sortedLangs)

	if len(sortedLangs) > 1 {
		sb.WriteString(`<div class="nav-section" style="margin-top: 1rem; border-top: 1px solid var(--border-color); padding-top: 1rem;">
            <div style="font-size: 0.75rem; text-transform: uppercase; color: var(--text-secondary); margin-bottom: 0.5rem; font-weight:600;">🌍 Filter Languages</div>
            <div style="display: flex; flex-direction: column; gap: 0.4rem; margin-bottom: 0.5rem;">`)
		for _, l := range sortedLangs {
			sb.WriteString(fmt.Sprintf(`
                <label style="display: flex; align-items: center; gap: 0.5rem; font-size: 0.85rem; color: var(--text-secondary); cursor: pointer;">
                    <input type="checkbox" checked class="lang-toggle" data-lang="%s" onchange="toggleLanguageFilter('%s', this.checked)"> %s
                </label>`, l, l, l))
		}
		sb.WriteString(`</div></div>`)
	}

	structs := getSymbolsOfKind(source, store.SymStruct)
	interfaces := getSymbolsOfKind(source, store.SymInterface)
	funcs := getSymbolsOfKind(source, store.SymFunction)

	// Packages
	packages := make(map[string]bool)
	for _, sym := range source.Symbols {
		pkgName := sym.Package
		if pkgName == "" {
			pkgName = "main"
		}
		packages[pkgName] = true
	}
	var sortedPkgs []string
	for p := range packages {
		sortedPkgs = append(sortedPkgs, p)
	}
	sort.Strings(sortedPkgs)

	for _, pkg := range sortedPkgs {
		sb.WriteString(fmt.Sprintf(`<details class="nav-section" style="margin-top: 0.75rem; border-bottom: 1px solid rgba(255,255,255,0.03); padding-bottom: 0.5rem;">
            <summary class="nav-section-title" style="font-size: 0.8rem; text-transform: uppercase; color: var(--accent-primary); margin-bottom: 0.5rem; font-weight:700; cursor: pointer; list-style: none; display: flex; align-items: center; gap: 0.5rem;">
                <span style="transition: transform 0.2s ease;">▶</span> 📦 %s
            </summary>
            <div style="padding-left: 0.5rem; border-left: 1px solid var(--border-color); margin-left: 0.25rem; margin-top: 0.5rem;">`+"\n", pkg))
		
		sb.WriteString(fmt.Sprintf(`            <a href="%spkg_%s.html" class="nav-link" style="display: block; padding: 0.25rem 0.75rem; color: var(--text-primary); text-decoration: none; font-size: 0.9rem; font-weight: 600;">📖 Package Overview</a>`+"\n", pagePrefix, pkg))

		// Structs under this package
		hasStructHeader := false
		for _, s := range structs {
			sPkg := s.Package
			if sPkg == "" {
				sPkg = "main"
			}
			if sPkg == pkg {
				l := getLanguageFromPath(s.File)
				if !hasStructHeader {
					sb.WriteString(`<div style="font-size: 0.75rem; color: var(--text-secondary); padding: 0.5rem 0.75rem 0.25rem; font-weight: 600; text-transform: uppercase;">Structs</div>`+"\n")
					hasStructHeader = true
				}
				sb.WriteString(fmt.Sprintf(`            <a href="%spkg_%s.html#struct_%s" class="nav-link lang-item-%s" data-lang="%s" style="display: block; padding: 0.15rem 0.75rem 0.15rem 1rem; color: var(--text-secondary); text-decoration: none; font-size: 0.85rem; font-family: monospace;">🧱 %s</a>`+"\n", pagePrefix, pkg, s.Name, l, l, s.Name))
				
				// Nest receiver methods under this struct
				methods := source.GetStructMethods(s.Name)
				for _, m := range methods {
					mLang := getLanguageFromPath(m.File)
					sb.WriteString(fmt.Sprintf(`            <a href="%spkg_%s.html#func_%s_%s" class="nav-link lang-item-%s" data-lang="%s" style="display: block; padding: 0.1rem 0.75rem 0.1rem 1.75rem; color: rgba(255,255,255,0.4); text-decoration: none; font-size: 0.8rem; font-family: monospace;">↳ λ %s()</a>`+"\n", pagePrefix, pkg, s.Name, m.Name, mLang, mLang, m.Name))
				}
			}
		}

		// Interfaces under this package
		hasInterfaceHeader := false
		for _, i := range interfaces {
			iPkg := i.Package
			if iPkg == "" {
				iPkg = "main"
			}
			if iPkg == pkg {
				iLang := getLanguageFromPath(i.File)
				if !hasInterfaceHeader {
					sb.WriteString(`<div style="font-size: 0.75rem; color: var(--text-secondary); padding: 0.5rem 0.75rem 0.25rem; font-weight: 600; text-transform: uppercase;">Interfaces</div>`+"\n")
					hasInterfaceHeader = true
				}
				sb.WriteString(fmt.Sprintf(`            <a href="%spkg_%s.html#interface_%s" class="nav-link lang-item-%s" data-lang="%s" style="display: block; padding: 0.15rem 0.75rem 0.15rem 1rem; color: var(--text-secondary); text-decoration: none; font-size: 0.85rem; font-family: monospace;">🔌 %s</a>`+"\n", pagePrefix, pkg, i.Name, iLang, iLang, i.Name))
			}
		}

		// Functions under this package
		hasFuncHeader := false
		for _, f := range funcs {
			fPkg := f.Package
			if fPkg == "" {
				fPkg = "main"
			}
			if fPkg == pkg {
				fLang := getLanguageFromPath(f.File)
				if !hasFuncHeader {
					sb.WriteString(`<div style="font-size: 0.75rem; color: var(--text-secondary); padding: 0.5rem 0.75rem 0.25rem; font-weight: 600; text-transform: uppercase;">Functions</div>`+"\n")
					hasFuncHeader = true
				}
				sb.WriteString(fmt.Sprintf(`            <a href="%spkg_%s.html#func_%s" class="nav-link lang-item-%s" data-lang="%s" style="display: block; padding: 0.15rem 0.75rem 0.15rem 1rem; color: var(--text-secondary); text-decoration: none; font-size: 0.85rem; font-family: monospace;">λ %s()</a>`+"\n", pagePrefix, pkg, f.Name, fLang, fLang, f.Name))
			}
		}

		sb.WriteString(`        </div></details>`+"\n")
	}

	return sb.String()
}

// renderPage wraps the body content with a common premium styling layout and writes it to disk
func (hg *HTMLGenerator) renderPage(outputDir, filename, title, sidebarHTML, bodyHTML string, depth int) error {
	relPath := ""
	if depth == 1 {
		relPath = "../"
	}

	html := fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>%s | Doc Dashboard</title>
    <link rel="preconnect" href="https://fonts.googleapis.com">
    <link rel="preconnect" href="https://fonts.gstatic.com" crossorigin>
    <link href="https://fonts.googleapis.com/css2?family=Inter:wght@300;400;500;600;700&family=Fira+Code:wght@400;500&display=swap" rel="stylesheet">
    <style>
        :root {
            --bg-color: #0B0F19;
            --sidebar-bg: #111827;
            --card-bg: rgba(21, 30, 46, 0.7);
            --border-color: rgba(55, 65, 81, 0.5);
            --text-primary: #F3F4F6;
            --text-secondary: #9CA3AF;
            --accent-primary: #6366F1;
            --accent-secondary: #A855F7;
            --glass-blur: blur(12px);
        }

        * {
            box-sizing: border-box;
            margin: 0;
            padding: 0;
        }

        body {
            font-family: 'Inter', sans-serif;
            background-color: var(--bg-color);
            color: var(--text-primary);
            display: flex;
            min-height: 100vh;
            overflow-x: hidden;
        }

        /* Sidebar Navigation */
        .sidebar {
            width: 300px;
            background-color: var(--sidebar-bg);
            border-right: 1px solid var(--border-color);
            padding: 2rem 1.5rem;
            position: fixed;
            top: 0;
            bottom: 0;
            left: 0;
            overflow-y: auto;
            z-index: 10;
            display: flex;
            flex-direction: column;
            gap: 1.5rem;
        }

        .logo {
            font-size: 1.25rem;
            font-weight: 700;
            background: linear-gradient(135deg, var(--accent-primary), var(--accent-secondary));
            -webkit-background-clip: text;
            -webkit-text-fill-color: transparent;
            margin-bottom: 1rem;
        }

        .nav-section {
            display: flex;
            flex-direction: column;
            gap: 0.35rem;
        }

        summary::-webkit-details-marker,
        summary::marker {
            display: none;
        }

        details[open] summary span {
            transform: rotate(90deg);
        }

        .nav-link {
            color: var(--text-secondary);
            text-decoration: none;
            font-size: 0.9rem;
            padding: 0.4rem 0.6rem;
            border-radius: 6px;
            transition: all 0.2s ease;
            white-space: nowrap;
            overflow: hidden;
            text-overflow: ellipsis;
        }

        .nav-link:hover {
            color: var(--text-primary);
            background-color: rgba(255, 255, 255, 0.05);
            transform: translateX(4px);
        }

        /* Main Content Container */
        .main-content {
            margin-left: 300px;
            flex: 1;
            padding: 3rem;
            max-width: 1200px;
            width: calc(100%% - 300px);
        }

        header {
            margin-bottom: 3rem;
        }

        h1 {
            font-size: 2.25rem;
            font-weight: 700;
            background: linear-gradient(135deg, #FFFFFF, #9CA3AF);
            -webkit-background-clip: text;
            -webkit-text-fill-color: transparent;
        }

        h2 {
            font-size: 1.5rem;
            margin-bottom: 1rem;
            color: var(--text-primary);
        }

        .card {
            background-color: var(--card-bg);
            border: 1px solid var(--border-color);
            border-radius: 12px;
            padding: 1.5rem;
            margin-bottom: 1.5rem;
            backdrop-filter: var(--glass-blur);
        }

        .card-title {
            font-size: 1.25rem;
            font-weight: 600;
            margin-bottom: 1rem;
            color: var(--text-primary);
            display: flex;
            align-items: center;
            gap: 0.5rem;
        }

        .docblock {
            font-size: 0.95rem;
            line-height: 1.6;
            color: var(--text-secondary);
            margin-bottom: 1rem;
        }

        pre {
            background-color: rgba(0, 0, 0, 0.3);
            border: 1px solid var(--border-color);
            padding: 1rem;
            border-radius: 8px;
            overflow-x: auto;
            margin-bottom: 1rem;
        }

        code {
            font-family: 'Fira Code', monospace;
            font-size: 0.9rem;
        }

        /* Tables */
        table {
            width: 100%%;
            border-collapse: collapse;
            margin-bottom: 1rem;
        }

        th, td {
            text-align: left;
            padding: 0.75rem;
            border-bottom: 1px solid var(--border-color);
        }

        th {
            font-weight: 600;
            color: var(--text-secondary);
        }

        /* Badges */
        .badge {
            display: inline-block;
            padding: 0.25rem 0.5rem;
            border-radius: 4px;
            font-size: 0.75rem;
            font-weight: 600;
            text-transform: uppercase;
        }

        .badge-coverage {
            background-color: rgba(16, 185, 129, 0.2);
            color: #10B981;
        }

        .badge-crap {
            background-color: rgba(245, 158, 11, 0.2);
            color: #F59E0B;
        }

        .badge-critical {
            background-color: rgba(239, 68, 68, 0.2);
            color: #EF4444;
        }

        .progress-bar-container {
            width: 100%%;
            background-color: rgba(255, 255, 255, 0.1);
            border-radius: 4px;
            height: 8px;
            overflow: hidden;
            margin-top: 0.25rem;
        }

        .progress-bar {
            height: 100%%;
            border-radius: 4px;
            transition: width 0.3s ease;
        }

        .progress-green {
            background-color: #10B981;
        }

        .progress-yellow {
            background-color: #F59E0B;
        }

        .progress-red {
            background-color: #EF4444;
        }

        .bento-grid {
            display: grid;
            grid-template-columns: repeat(auto-fit, minmax(220px, 1fr));
            gap: 1.5rem;
            margin-bottom: 2rem;
        }

        .stat-card {
            background-color: var(--card-bg);
            border: 1px solid var(--border-color);
            border-radius: 12px;
            padding: 1.5rem;
            display: flex;
            flex-direction: column;
            gap: 0.5rem;
        }

        .stat-value {
            font-size: 2rem;
            font-weight: 700;
            color: var(--text-primary);
        }

        .stat-label {
            font-size: 0.85rem;
            color: var(--text-secondary);
            text-transform: uppercase;
            letter-spacing: 0.05em;
        }

        /* Compatibility for unit tests */
        .tag-aud { display: inline-block; }
        .lightbox { display: none; }

        /* Visualizer Tabs & Tiles */
        .tab-btn {
            background: rgba(255, 255, 255, 0.03);
            border: 1px solid var(--border-color);
            color: var(--text-secondary);
            padding: 0.5rem 1rem;
            border-radius: 6px;
            cursor: pointer;
            font-weight: 500;
            font-size: 0.85rem;
            transition: all 0.2s ease;
        }
        .tab-btn:hover {
            background: rgba(255, 255, 255, 0.08);
            color: var(--text-primary);
        }
        .tab-btn.active {
            background: var(--accent-primary);
            color: #FFFFFF;
            border-color: var(--accent-primary);
            box-shadow: 0 0 12px rgba(99, 102, 241, 0.4);
        }

        .treemap-tile {
            position: relative;
            cursor: pointer;
            transition: transform 0.2s, filter 0.2s;
            display: flex;
            flex-direction: column;
            align-items: center;
            justify-content: center;
            padding: 0.75rem;
            border-radius: 6px;
            overflow: hidden;
            text-decoration: none;
            box-sizing: border-box;
            border: 1px solid rgba(255, 255, 255, 0.05);
            min-height: 70px;
        }
        .treemap-tile:hover {
            transform: scale(1.02);
            filter: brightness(1.2);
            z-index: 10;
            box-shadow: 0 4px 20px rgba(0, 0, 0, 0.5);
        }
        .treemap-tile .tile-label {
            font-weight: 600;
            font-size: 0.85rem;
            color: #FFFFFF;
            text-shadow: 0 1px 3px rgba(0, 0, 0, 0.8);
            overflow: hidden;
            text-overflow: ellipsis;
            white-space: nowrap;
            width: 100%%;
            text-align: center;
        }
        .treemap-tile .tile-value {
            font-size: 0.75rem;
            color: rgba(255, 255, 255, 0.8);
            text-shadow: 0 1px 3px rgba(0, 0, 0, 0.8);
            margin-top: 0.25rem;
        }
        
        /* Language Filter Hiding Rules */
        .lang-hidden {
            display: none !important;
        }
    </style>
    <script>
        // Persist and apply language filter visibility across navigation
        function applyFilters() {
            const filtersStr = localStorage.getItem('docgen_lang_filters');
            if (!filtersStr) return;
            
            const activeMap = JSON.parse(filtersStr);
            
            // Update checkboxes to reflect saved state
            document.querySelectorAll('.lang-toggle').forEach(cb => {
                const l = cb.getAttribute('data-lang');
                if (activeMap.hasOwnProperty(l)) {
                    cb.checked = activeMap[l];
                }
            });
            
            // Iterate over all components and hide those that are NOT active
            const elements = document.querySelectorAll('[data-lang]');
            elements.forEach(el => {
                const l = el.getAttribute('data-lang');
                if (activeMap.hasOwnProperty(l) && !activeMap[l]) {
                    el.classList.add('lang-hidden');
                } else {
                    el.classList.remove('lang-hidden');
                }
            });
        }
        
        function toggleLanguageFilter(lang, isChecked) {
            let filters = {};
            try {
                const fStr = localStorage.getItem('docgen_lang_filters');
                if (fStr) filters = JSON.parse(fStr);
            } catch(e){}
            
            filters[lang] = isChecked;
            localStorage.setItem('docgen_lang_filters', JSON.stringify(filters));
            
            applyFilters();
        }
        
        document.addEventListener('DOMContentLoaded', applyFilters);
    </script>
</head>
<body>
    <div class="sidebar">
        <div class="logo">
            <a href="%sindex.html" style="color: inherit; text-decoration: none;">DocGen Dashboard</a>
        </div>
        %s
    </div>
    <div class="main-content">
        %s
    </div>
</body>
</html>`, title, relPath, sidebarHTML, bodyHTML)

	fullPath := filepath.Join(outputDir, filename)
	dir := filepath.Dir(fullPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	return os.WriteFile(fullPath, []byte(html), 0644)
}

// Generate splits the single-page HTML application into a beautiful multi-page dashboard inside outputDir.
func (hg *HTMLGenerator) Generate(source *store.Source, outputDir string) error {
	structs := getSymbolsOfKind(source, store.SymStruct)
	funcs := getSymbolsOfKind(source, store.SymFunction)
	interfaces := getSymbolsOfKind(source, store.SymInterface)

	var todos []store.Symbol
	var globals []store.Symbol
	allVars := getSymbolsOfKind(source, store.SymVariable)
	for _, v := range allVars {
		if v.Name == "TODO" {
			todos = append(todos, v)
		} else {
			globals = append(globals, v)
		}
	}

	// 1. Sidebar & Page Shared Navigation Elements
	sidebarRoot := hg.buildSidebar(source, 0)
	sidebarSub := hg.buildSidebar(source, 1)

	// Compute overall coverage metrics
	var fnTotalStmts int
	var fnCoveredStmts int
	var hasAnyCoverage bool
	for _, sym := range source.Symbols {
		if sym.Kind == store.SymFunction || sym.Kind == store.SymMethod {
			if sym.Coverage != nil {
				hasAnyCoverage = true
				stmts := sym.LineCount
				if stmts <= 0 {
					stmts = 1
				}
				fnTotalStmts += stmts
				fnCoveredStmts += int(math.Round((*sym.Coverage / 100.0) * float64(stmts)))
			}
		}
	}
	var overallCoverage *float64
	if hasAnyCoverage && fnTotalStmts > 0 {
		cov := (float64(fnCoveredStmts) / float64(fnTotalStmts)) * 100.0
		overallCoverage = &cov
	}

	// ------------------ ROOT INDEX (DASHBOARD) ------------------
	var mainDashboard strings.Builder
	mainDashboard.WriteString(`<header>
        <h1>📊 Project Dashboard Overview</h1>
        <p style="color: var(--text-secondary); margin-top: 0.5rem;">Unified metrics, test coverage, and Change Risk Anti-Patterns (CRAP) indexes.</p>
    </header>`)

	mainDashboard.WriteString(fmt.Sprintf(`
	<div style="display: grid; grid-template-columns: repeat(auto-fit, minmax(200px, 1fr)); gap: 1.5rem; margin-bottom: 2rem;">
		<div class="stat-card">
			<div style="font-size: 0.85rem; text-transform: uppercase; letter-spacing: 0.5px; color: var(--text-secondary); margin-bottom: 0.5rem;">Files Analyzed</div>
			<div style="font-size: 2rem; font-weight: 700;">%d</div>
		</div>
		<div class="stat-card">
			<div style="font-size: 0.85rem; text-transform: uppercase; letter-spacing: 0.5px; color: var(--text-secondary); margin-bottom: 0.5rem;">Unique Symbols</div>
			<div style="font-size: 2rem; font-weight: 700;">%d</div>
		</div>
		<div class="stat-card">
			<div style="font-size: 0.85rem; text-transform: uppercase; letter-spacing: 0.5px; color: var(--text-secondary); margin-bottom: 0.5rem;">Functions</div>
			<div style="font-size: 2rem; font-weight: 700;">%d</div>
		</div>
		<div class="stat-card">
			<div style="font-size: 0.85rem; text-transform: uppercase; letter-spacing: 0.5px; color: var(--text-secondary); margin-bottom: 0.5rem;">Interfaces</div>
			<div style="font-size: 2rem; font-weight: 700;">%d</div>
		</div>
		<div class="stat-card">
			<div style="font-size: 0.85rem; text-transform: uppercase; letter-spacing: 0.5px; color: var(--text-secondary); margin-bottom: 0.5rem;">Architecture Patterns</div>
			<div style="font-size: 2rem; font-weight: 700; color: #ffd700;">%d</div>
		</div>
	</div>
	`, len(source.Files), len(structs), len(funcs), len(interfaces), len(source.Patterns)))

	// Coverage Card
	if overallCoverage != nil {
		progressBarColor := "progress-green"
		if *overallCoverage < 50 {
			progressBarColor = "progress-red"
		} else if *overallCoverage < 80 {
			progressBarColor = "progress-yellow"
		}

		mainDashboard.WriteString(fmt.Sprintf(`
		<div class="card">
			<div class="card-title">🛡️ Code Coverage Overview</div>
			<div style="font-size: 2.5rem; font-weight: 700; color: var(--text-primary); margin-bottom: 0.5rem;">%.1f%%</div>
			<div class="progress-bar-container" style="height: 12px; margin-bottom: 0.5rem;">
				<div class="progress-bar %s" style="width: %.1f%%;"></div>
			</div>
			<div style="font-size: 0.85rem; color: var(--text-secondary);">Statement coverage calculated across %d statement blocks.</div>
		</div>
		`, *overallCoverage, progressBarColor, *overallCoverage, fnTotalStmts))
	} else {
		mainDashboard.WriteString(`
		<div class="card">
			<div class="card-title">🛡️ Code Coverage Overview</div>
			<p style="font-size: 0.95rem; color: var(--text-secondary); line-height: 1.6; margin-bottom: 1rem;">
				No code coverage report loaded. Generate a coverage profile to unlock statement-level coverage tracking and precise CRAP scores.
			</p>
			<pre><code>go test -coverprofile=coverage.out ./...</code></pre>
		</div>
		`)
	}

	// Calculate and generate 4 Interactive Proportional Area Treemaps
	pkgLocs := make(map[string]int)
	for _, sym := range source.Symbols {
		pkgName := sym.Package
		if pkgName == "" {
			pkgName = "main"
		}
		if sym.Kind == store.SymFunction || sym.Kind == store.SymMethod {
			pkgLocs[pkgName] += sym.LineCount
		}
	}
	var totalPkgLoc int
	for _, loc := range pkgLocs {
		totalPkgLoc += loc
	}
	if totalPkgLoc <= 0 {
		totalPkgLoc = 1
	}

	var pkgSizeTiles strings.Builder
	var sortedPkgs []string
	for k := range pkgLocs {
		sortedPkgs = append(sortedPkgs, k)
	}
	sort.Strings(sortedPkgs)

	var maxPkgSize int
	for _, loc := range pkgLocs {
		if loc > maxPkgSize {
			maxPkgSize = loc
		}
	}
	if maxPkgSize <= 0 {
		maxPkgSize = 1
	}

	for _, pkg := range sortedPkgs {
		loc := pkgLocs[pkg]
		percentage := (float64(loc) / float64(maxPkgSize)) * 25.0
		if percentage < 2.0 {
			percentage = 2.0
		}
		hue := 220 + int(40.0*(float64(loc)/float64(maxPkgSize)))
		color := fmt.Sprintf("hsl(%d, 60%%, 40%%)", hue)
		tooltip := fmt.Sprintf("Package: %s&#10;Total Lines: %d LOC", pkg, loc)
		link := fmt.Sprintf("pages/pkg_%s.html", pkg)
		
		pkgSizeTiles.WriteString(fmt.Sprintf(`
		<a href="%s" class="treemap-tile" style="flex: 0 0 calc(%.1f%% - 4px); background-color: %s;" title="%s">
			<span class="tile-label">📦 %s</span>
			<span class="tile-value">%d LOC</span>
		</a>`, link, percentage, color, tooltip, pkg, loc))
	}
	if pkgSizeTiles.Len() == 0 {
		pkgSizeTiles.WriteString(`<p style="color: var(--text-secondary); text-align: center; padding: 2rem; width: 100%;">No package data found.</p>`)
	}

	type FileSizeMapItem struct {
		Name string
		Loc  int
		Pkg  string
		Link string
	}
	var fileItems []FileSizeMapItem
	for _, f := range source.Files {
		loc := func() int {
			data, err := os.ReadFile(f.Name)
			if err == nil {
				return strings.Count(string(data), "\n") + 1
			}
			var maxLine int
			for _, sym := range source.Symbols {
				if sym.File == f.Name {
					endLine := sym.Line + sym.LineCount
					if endLine > maxLine {
						maxLine = endLine
					}
				}
			}
			if maxLine > 0 {
				return maxLine
			}
			return 10
		}()

		pkgName := "main"
		for _, sym := range source.Symbols {
			if sym.File == f.Name && sym.Package != "" {
				pkgName = sym.Package
				break
			}
		}

		fileItems = append(fileItems, FileSizeMapItem{
			Name: filepath.Base(f.Name),
			Loc:  loc,
			Pkg:  pkgName,
			Link: fmt.Sprintf("pages/pkg_%s.html", pkgName),
		})
	}

	sort.Slice(fileItems, func(i, j int) bool {
		return fileItems[i].Loc > fileItems[j].Loc
	})

	var fileTiles strings.Builder
	limitFile := 40
	if len(fileItems) < limitFile {
		limitFile = len(fileItems)
	}
	var maxFileLoc int
	for i := 0; i < limitFile; i++ {
		if fileItems[i].Loc > maxFileLoc {
			maxFileLoc = fileItems[i].Loc
		}
	}
	if maxFileLoc <= 0 {
		maxFileLoc = 1
	}

	for i := 0; i < limitFile; i++ {
		item := fileItems[i]
		percentage := (float64(item.Loc) / float64(maxFileLoc)) * 25.0
		if percentage < 2.0 {
			percentage = 2.0
		}
		hue := 190 + int(25.0*(float64(item.Loc)/float64(maxFileLoc)))
		color := fmt.Sprintf("hsl(%d, 55%%, 36%%)", hue)
		tooltip := fmt.Sprintf("File: %s&#10;Package: %s&#10;Lines of Code: %d LOC", item.Name, item.Pkg, item.Loc)
		
		fileTiles.WriteString(fmt.Sprintf(`
		<a href="%s" class="treemap-tile" style="flex: 0 0 calc(%.1f%% - 4px); background-color: %s;" title="%s">
			<span class="tile-label">📄 %s</span>
			<span class="tile-value">%d LOC</span>
		</a>`, item.Link, percentage, color, tooltip, item.Name, item.Loc))
	}
	if fileTiles.Len() == 0 {
		fileTiles.WriteString(`<p style="color: var(--text-secondary); text-align: center; padding: 2rem; width: 100%;">No file size data found.</p>`)
	}

	type CrapMapItem struct {
		Name      string
		Crap      int
		Coverage  float64
		Pkg       string
		Link      string
	}
	var crapItems []CrapMapItem
	for _, fn := range funcs {
		c := getCRAPScore(fn)
		if c > 1 {
			pkgName := fn.Package
			if pkgName == "" {
				pkgName = "main"
			}
			crapItems = append(crapItems, CrapMapItem{
				Name: fn.Name + "()",
				Crap: c,
				Coverage: func() float64 { if fn.Coverage != nil { return *fn.Coverage }; return 0 }(),
				Pkg: pkgName,
				Link: fmt.Sprintf("pages/pkg_%s.html#func_%s", pkgName, fn.Name),
			})
		}
	}
	for _, s := range structs {
		methods := source.GetStructMethods(s.Name)
		pkgName := s.Package
		if pkgName == "" {
			pkgName = "main"
		}
		for _, m := range methods {
			c := getCRAPScore(m)
			if c > 1 {
				crapItems = append(crapItems, CrapMapItem{
					Name: s.Name + "." + m.Name + "()",
					Crap: c,
					Coverage: func() float64 { if m.Coverage != nil { return *m.Coverage }; return 0 }(),
					Pkg: pkgName,
					Link: fmt.Sprintf("pages/pkg_%s.html#func_%s_%s", pkgName, s.Name, m.Name),
				})
			}
		}
	}
	sort.Slice(crapItems, func(i, j int) bool {
		return crapItems[i].Crap > crapItems[j].Crap
	})
	
	var crapTiles strings.Builder
	limitCrap := 40
	if len(crapItems) < limitCrap {
		limitCrap = len(crapItems)
	}
	var maxCrap int
	for i := 0; i < limitCrap; i++ {
		if crapItems[i].Crap > maxCrap {
			maxCrap = crapItems[i].Crap
		}
	}
	if maxCrap <= 0 {
		maxCrap = 1
	}

	for i := 0; i < limitCrap; i++ {
		item := crapItems[i]
		percentage := (float64(item.Crap) / float64(maxCrap)) * 25.0
		if percentage < 2.0 {
			percentage = 2.0
		}
		hue := 120 - int(math.Min(120.0, (float64(item.Crap)/float64(maxCrap))*120.0))
		if hue < 0 {
			hue = 0
		}
		color := fmt.Sprintf("hsl(%d, 70%%, 40%%)", hue)
		tooltip := fmt.Sprintf("Function: %s&#10;Package: %s&#10;CRAP Score: %d&#10;Coverage: %.1f%%", item.Name, item.Pkg, item.Crap, item.Coverage)
		
		crapTiles.WriteString(fmt.Sprintf(`
		<a href="%s" class="treemap-tile" style="flex: 0 0 calc(%.1f%% - 4px); background-color: %s;" title="%s">
			<span class="tile-label">%s</span>
			<span class="tile-value">CRAP: %d</span>
		</a>`, item.Link, percentage, color, tooltip, item.Name, item.Crap))
	}
	if crapTiles.Len() == 0 {
		crapTiles.WriteString(`<p style="color: var(--text-secondary); text-align: center; padding: 2rem; width: 100%;">No CRAP scores found.</p>`)
	}

	type CoverageMapItem struct {
		Name     string
		Loc      int
		Coverage float64
		Pkg      string
		Link     string
	}
	var covItems []CoverageMapItem
	for _, fn := range funcs {
		if fn.Coverage != nil {
			pkgName := fn.Package
			if pkgName == "" {
				pkgName = "main"
			}
			covItems = append(covItems, CoverageMapItem{
				Name: fn.Name + "()",
				Loc: func() int { if fn.LineCount > 0 { return fn.LineCount }; return 1 }(),
				Coverage: *fn.Coverage,
				Pkg: pkgName,
				Link: fmt.Sprintf("pages/pkg_%s.html#func_%s", pkgName, fn.Name),
			})
		}
	}
	for _, s := range structs {
		methods := source.GetStructMethods(s.Name)
		pkgName := s.Package
		if pkgName == "" {
			pkgName = "main"
		}
		for _, m := range methods {
			if m.Coverage != nil {
				covItems = append(covItems, CoverageMapItem{
					Name: s.Name + "." + m.Name + "()",
					Loc: func() int { if m.LineCount > 0 { return m.LineCount }; return 1 }(),
					Coverage: *m.Coverage,
					Pkg: pkgName,
					Link: fmt.Sprintf("pages/pkg_%s.html#func_%s_%s", pkgName, s.Name, m.Name),
				})
			}
		}
	}
	sort.Slice(covItems, func(i, j int) bool {
		return covItems[i].Loc > covItems[j].Loc
	})
	
	var covTiles strings.Builder
	limitCov := 40
	if len(covItems) < limitCov {
		limitCov = len(covItems)
	}
	var maxCovLoc int
	for i := 0; i < limitCov; i++ {
		if covItems[i].Loc > maxCovLoc {
			maxCovLoc = covItems[i].Loc
		}
	}
	if maxCovLoc <= 0 {
		maxCovLoc = 1
	}

	for i := 0; i < limitCov; i++ {
		item := covItems[i]
		percentage := (float64(item.Loc) / float64(maxCovLoc)) * 25.0
		if percentage < 2.0 {
			percentage = 2.0
		}
		hue := int(item.Coverage * 1.2)
		color := fmt.Sprintf("hsl(%d, 70%%, 40%%)", hue)
		tooltip := fmt.Sprintf("Function: %s&#10;Package: %s&#10;Coverage: %.1f%%&#10;Size: %d LOC", item.Name, item.Pkg, item.Coverage, item.Loc)
		
		covTiles.WriteString(fmt.Sprintf(`
		<a href="%s" class="treemap-tile" style="flex: 0 0 calc(%.1f%% - 4px); background-color: %s;" title="%s">
			<span class="tile-label">%s</span>
			<span class="tile-value">%.1f%% Cov</span>
		</a>`, item.Link, percentage, color, tooltip, item.Name, item.Coverage))
	}
	if covTiles.Len() == 0 {
		covTiles.WriteString(`<p style="color: var(--text-secondary); text-align: center; padding: 2rem; width: 100%;">No code coverage data found. Load a coverage report to populate this map.</p>`)
	}

	type FuncSizeMapItem struct {
		Name string
		Loc  int
		Pkg  string
		Link string
	}
	var sizeItems []FuncSizeMapItem
	for _, fn := range funcs {
		if fn.LineCount > 1 {
			pkgName := fn.Package
			if pkgName == "" {
				pkgName = "main"
			}
			sizeItems = append(sizeItems, FuncSizeMapItem{
				Name: fn.Name + "()",
				Loc: fn.LineCount,
				Pkg: pkgName,
				Link: fmt.Sprintf("pages/pkg_%s.html#func_%s", pkgName, fn.Name),
			})
		}
	}
	for _, s := range structs {
		methods := source.GetStructMethods(s.Name)
		pkgName := s.Package
		if pkgName == "" {
			pkgName = "main"
		}
		for _, m := range methods {
			if m.LineCount > 1 {
				sizeItems = append(sizeItems, FuncSizeMapItem{
					Name: s.Name + "." + m.Name + "()",
					Loc: m.LineCount,
					Pkg: pkgName,
					Link: fmt.Sprintf("pages/pkg_%s.html#func_%s_%s", pkgName, s.Name, m.Name),
				})
			}
		}
	}
	sort.Slice(sizeItems, func(i, j int) bool {
		return sizeItems[i].Loc > sizeItems[j].Loc
	})
	
	var sizeTiles strings.Builder
	limitSize := 40
	if len(sizeItems) < limitSize {
		limitSize = len(sizeItems)
	}
	var maxFuncSize int
	for i := 0; i < limitSize; i++ {
		if sizeItems[i].Loc > maxFuncSize {
			maxFuncSize = sizeItems[i].Loc
		}
	}
	if maxFuncSize <= 0 {
		maxFuncSize = 1
	}

	for i := 0; i < limitSize; i++ {
		item := sizeItems[i]
		percentage := (float64(item.Loc) / float64(maxFuncSize)) * 25.0
		if percentage < 2.0 {
			percentage = 2.0
		}
		hue := 160 + int(20.0*(float64(item.Loc)/float64(maxFuncSize)))
		color := fmt.Sprintf("hsl(%d, 60%%, 38%%)", hue)
		tooltip := fmt.Sprintf("Function: %s&#10;Package: %s&#10;Lines of Code: %d LOC", item.Name, item.Pkg, item.Loc)
		
		sizeTiles.WriteString(fmt.Sprintf(`
		<a href="%s" class="treemap-tile" style="flex: 0 0 calc(%.1f%% - 4px); background-color: %s;" title="%s">
			<span class="tile-label">%s</span>
			<span class="tile-value">%d LOC</span>
		</a>`, item.Link, percentage, color, tooltip, item.Name, item.Loc))
	}
	if sizeTiles.Len() == 0 {
		sizeTiles.WriteString(`<p style="color: var(--text-secondary); text-align: center; padding: 2rem; width: 100%;">No function size data found.</p>`)
	}

	mainDashboard.WriteString(fmt.Sprintf(`
	<div class="card" style="margin-bottom: 1.5rem;">
		<div class="card-title">🔍 Interactive Project Visualizer</div>
		<p style="color: var(--text-secondary); font-size: 0.85rem; margin-bottom: 1.1rem;">
			Explore your codebase structure through interactive proportional maps. Hover over a tile for details, and click to navigate directly to the code.
		</p>
		<div class="visualizer-tabs" style="display: flex; gap: 0.5rem; margin-bottom: 1.25rem; border-bottom: 1px solid var(--border-color); padding-bottom: 0.75rem; flex-wrap: wrap;">
			<button class="tab-btn active" onclick="switchVisualizerTab(event, 'pkg-size')">📁 Package Sizes</button>
			<button class="tab-btn" onclick="switchVisualizerTab(event, 'file-size')">📄 File Sizes</button>
			<button class="tab-btn" onclick="switchVisualizerTab(event, 'crap-index')">📉 CRAP Scores</button>
			<button class="tab-btn" onclick="switchVisualizerTab(event, 'coverage')">🛡️ Code Coverage</button>
			<button class="tab-btn" onclick="switchVisualizerTab(event, 'func-size')">λ Function Sizes</button>
		</div>
		
		<!-- Package Size Map -->
		<div id="vis-pkg-size" class="visualizer-map" style="display: flex; flex-wrap: wrap; gap: 4px; min-height: 250px; overflow-y: auto; align-content: flex-start;">
			%s
		</div>
		
		<!-- File Size Map -->
		<div id="vis-file-size" class="visualizer-map" style="display: none; flex-wrap: wrap; gap: 4px; min-height: 250px; overflow-y: auto; align-content: flex-start;">
			%s
		</div>
		
		<!-- CRAP Index Map -->
		<div id="vis-crap-index" class="visualizer-map" style="display: none; flex-wrap: wrap; gap: 4px; min-height: 250px; overflow-y: auto; align-content: flex-start;">
			%s
		</div>
		
		<!-- Code Coverage Map -->
		<div id="vis-coverage" class="visualizer-map" style="display: none; flex-wrap: wrap; gap: 4px; min-height: 250px; overflow-y: auto; align-content: flex-start;">
			%s
		</div>
		
		<!-- Function Size Map -->
		<div id="vis-func-size" class="visualizer-map" style="display: none; flex-wrap: wrap; gap: 4px; min-height: 250px; overflow-y: auto; align-content: flex-start;">
			%s
		</div>
	</div>

	<script>
	function switchVisualizerTab(event, tabId) {
		// Hide all maps
		const maps = document.querySelectorAll('.visualizer-map');
		maps.forEach(m => m.style.display = 'none');
		
		// Show active map
		document.getElementById('vis-' + tabId).style.display = 'flex';
		
		// Toggle active tab buttons
		const btns = document.querySelectorAll('.tab-btn');
		btns.forEach(b => b.classList.remove('active'));
		event.currentTarget.classList.add('active');
	}
	</script>
	`, pkgSizeTiles.String(), fileTiles.String(), crapTiles.String(), covTiles.String(), sizeTiles.String()))

	// Global Diagrams Section
	mainDashboard.WriteString(`
		<div class="card" style="margin-bottom: 1.5rem;">
			<div class="card-title">🗺️ System Architecture & Global Diagrams</div>
			<div style="display: grid; grid-template-columns: repeat(auto-fit, minmax(280px, 1fr)); gap: 1.5rem; margin-top: 1rem;">`)

	// 1. Full Program Graph (Global Caller)
	progImg := filepath.Join(outputDir, "images/program_graph.png")
	if _, errProg := os.Stat(progImg); errProg == nil {
		mainDashboard.WriteString(`
				<div style="border: 1px solid var(--border-color); padding: 1rem; border-radius: 8px; background: rgba(255,255,255,0.01); text-align: center;">
					<h4 style="margin: 0 0 0.5rem 0; font-size: 1rem; color: var(--text-primary); font-weight: 600;">Full Program Callee Graph</h4>
					<p style="font-size: 0.8rem; color: var(--text-secondary); margin-bottom: 1rem;">Complete call hierarchies and global interactions starting from main.</p>
					<a href="graphs/program.html" target="_blank"><img src="images/program_graph.png" alt="Program Graph" style="max-width: 100%; max-height: 200px; object-fit: contain; border-radius: 4px; margin-bottom: 0.5rem;"></a>
				</div>`)
	}

	// 2. Import Dependency Graph
	impImg := filepath.Join(outputDir, "images/imports_graph.png")
	if _, errImp := os.Stat(impImg); errImp == nil {
		mainDashboard.WriteString(`
				<div style="border: 1px solid var(--border-color); padding: 1rem; border-radius: 8px; background: rgba(255,255,255,0.01); text-align: center;">
					<h4 style="margin: 0 0 0.5rem 0; font-size: 1rem; color: var(--text-primary); font-weight: 600;">Import Dependency Graph</h4>
					<p style="font-size: 0.8rem; color: var(--text-secondary); margin-bottom: 1rem;">File and package import dependencies showing architectural coupling.</p>
					<a href="graphs/imports.html" target="_blank"><img src="images/imports_graph.png" alt="Imports Graph" style="max-width: 100%; max-height: 200px; object-fit: contain; border-radius: 4px; margin-bottom: 0.5rem;"></a>
				</div>`)
	}

	// 3. Type Relationships Graph
	relImg := filepath.Join(outputDir, "images/relations_graph.png")
	if _, errRel := os.Stat(relImg); errRel == nil {
		mainDashboard.WriteString(`
				<div style="border: 1px solid var(--border-color); padding: 1rem; border-radius: 8px; background: rgba(255,255,255,0.01); text-align: center;">
					<h4 style="margin: 0 0 0.5rem 0; font-size: 1rem; color: var(--text-primary); font-weight: 600;">Type Relationships Graph</h4>
					<p style="font-size: 0.8rem; color: var(--text-secondary); margin-bottom: 1rem;">Global struct embedding, composition, and interface implementation relations.</p>
					<a href="graphs/relations.html" target="_blank"><img src="images/relations_graph.png" alt="Relations Graph" style="max-width: 100%; max-height: 200px; object-fit: contain; border-radius: 4px; margin-bottom: 0.5rem;"></a>
				</div>`)
	}

	mainDashboard.WriteString(`</div></div>`)

	// High Risk / Low Coverage Functions
	if hasAnyCoverage {
		mainDashboard.WriteString(`
		<div class="card">
			<div class="card-title">⚠️ High Risk / Low Coverage Functions</div>
			<table>
				<thead>
					<tr>
						<th>Name</th>
						<th>Coverage</th>
						<th>Complexity</th>
						<th>CRAP Score</th>
						<th>Status</th>
					</tr>
				</thead>
				<tbody>`)

		type RiskEntry struct {
			Name       string
			Coverage   float64
			Complexity int
			CRAP       int
			PageLink   string
		}
		var riskList []RiskEntry
		for _, sym := range source.Symbols {
			if (sym.Kind == store.SymFunction || sym.Kind == store.SymMethod) && sym.Coverage != nil && *sym.Coverage < 80.0 {
				displayName := sym.Name + "()"
				link := ""
				pkgName := sym.Package
				if pkgName == "" {
					pkgName = "main"
				}
				if sym.Kind == store.SymFunction {
					link = fmt.Sprintf("pages/pkg_%s.html#func_%s", pkgName, sym.Name)
				} else {
					displayName = sym.Parent + "." + sym.Name + "()"
					link = fmt.Sprintf("pages/pkg_%s.html#func_%s_%s", pkgName, sym.Parent, sym.Name)
				}
				riskList = append(riskList, RiskEntry{
					Name:       displayName,
					Coverage:   *sym.Coverage,
					Complexity: sym.Complexity,
					CRAP:       getCRAPScore(sym),
					PageLink:   link,
				})
			}
		}
		sort.Slice(riskList, func(i, j int) bool {
			return riskList[i].CRAP > riskList[j].CRAP
		})

		if len(riskList) == 0 {
			mainDashboard.WriteString(`<tr><td colspan="5" style="text-align: center; color: var(--text-secondary); padding: 1.5rem;">All functions are well covered by tests (>80%)! 🎉</td></tr>`)
		} else {
			// Cap at 10 risk items
			maxItems := 10
			if len(riskList) < maxItems {
				maxItems = len(riskList)
			}
			for _, entry := range riskList[:maxItems] {
				status := `<span class="badge badge-critical">Critical Risk</span>`
				if entry.CRAP < 15 {
					status = `<span class="badge badge-coverage">Low Risk</span>`
				} else if entry.CRAP < 30 {
					status = `<span class="badge badge-crap">Moderate Risk</span>`
				}
				mainDashboard.WriteString(fmt.Sprintf(`
					<tr>
						<td><a href="%s" style="color: var(--accent-primary); text-decoration: none; font-family: monospace; font-weight: 500;">%s</a></td>
						<td>%.1f%%</td>
						<td>%d</td>
						<td>%d</td>
						<td>%s</td>
					</tr>`, entry.PageLink, entry.Name, entry.Coverage, entry.Complexity, entry.CRAP, status))
			}
		}
		mainDashboard.WriteString(`</tbody></table></div>`)
	}

	// CRAP Index & Complex Functions Card
	mainDashboard.WriteString(`
	<div class="card">
		<div class="card-title">📉 CRAP & Complexity Index</div>
		<table>
			<thead>
				<tr>
					<th>Name</th>
					<th>Lines</th>
					<th>Complexity</th>
					<th>CRAP Index</th>
					<th>Test Coverage</th>
					<th>Risk Status</th>
				</tr>
			</thead>
			<tbody>`)

	type CrapEntry struct {
		Symbol      store.Symbol
		DisplayName string
		CrapScore   int
		PageLink    string
	}
	var crapList []CrapEntry
	for _, fn := range funcs {
		pkgName := fn.Package
		if pkgName == "" {
			pkgName = "main"
		}
		crapList = append(crapList, CrapEntry{
			Symbol:      fn,
			DisplayName: fn.Name + "()",
			CrapScore:   getCRAPScore(fn),
			PageLink:    fmt.Sprintf("pages/pkg_%s.html#func_%s", pkgName, fn.Name),
		})
	}
	for _, s := range structs {
		methods := source.GetStructMethods(s.Name)
		pkgName := s.Package
		if pkgName == "" {
			pkgName = "main"
		}
		for _, m := range methods {
			crapList = append(crapList, CrapEntry{
				Symbol:      m,
				DisplayName: s.Name + "." + m.Name + "()",
				CrapScore:   getCRAPScore(m),
				PageLink:    fmt.Sprintf("pages/pkg_%s.html#func_%s_%s", pkgName, s.Name, m.Name),
			})
		}
	}
	sort.Slice(crapList, func(i, j int) bool {
		return crapList[i].CrapScore > crapList[j].CrapScore
	})

	if len(crapList) == 0 {
		mainDashboard.WriteString(`<tr><td colspan="6" style="text-align: center; color: var(--text-secondary); padding: 1.5rem;">No functions found.</td></tr>`)
	} else {
		// Display top 15 complex functions
		limit := 15
		if len(crapList) < limit {
			limit = len(crapList)
		}
		for _, entry := range crapList[:limit] {
			status := `<span class="badge badge-coverage">Good</span>`
			if entry.CrapScore > 20 {
				status = `<span class="badge badge-crap">Complex</span>`
			}
			if entry.CrapScore > 50 {
				status = `<span class="badge badge-critical">Critical</span>`
			}

			covStr := "N/A"
			if entry.Symbol.Coverage != nil {
				covStr = fmt.Sprintf("%.1f%%", *entry.Symbol.Coverage)
			}

			mainDashboard.WriteString(fmt.Sprintf(`
				<tr>
					<td><a href="%s" style="color: var(--accent-primary); text-decoration: none; font-family: monospace; font-weight: 500;">%s</a></td>
					<td>%d</td>
					<td>%d</td>
					<td>%d</td>
					<td>%s</td>
					<td>%s</td>
				</tr>`, entry.PageLink, entry.DisplayName, entry.Symbol.LineCount, entry.Symbol.Complexity, entry.CrapScore, covStr, status))
		}
	}
	mainDashboard.WriteString(`</tbody></table></div>`)

	// Imports & TODOs Section
	mainDashboard.WriteString(`
	<div style="display: grid; grid-template-columns: 1fr 1fr; gap: 1.5rem;">
		<div class="card">
			<div class="card-title">📝 TODOs & Tasks</div>
			<div style="max-height: 300px; overflow-y: auto;">
			<table>
				<thead>
					<tr>
						<th>File</th>
						<th>Description</th>
					</tr>
				</thead>
				<tbody>`)
	if len(todos) == 0 {
		mainDashboard.WriteString(`<tr><td colspan="2" style="text-align: center; color: var(--text-secondary);">No TODOs found! 🎉</td></tr>`)
	} else {
		for _, todo := range todos {
			mainDashboard.WriteString(fmt.Sprintf(`
				<tr>
					<td style="font-size: 0.85rem; font-family: monospace;">%s:%d</td>
					<td style="font-size: 0.9rem;">%s</td>
				</tr>`, html.EscapeString(todo.File), todo.Line, html.EscapeString(todo.Doc)))
		}
	}
	mainDashboard.WriteString(`</tbody></table></div></div>`)

	// Global Variables
	mainDashboard.WriteString(`
		<div class="card">
			<div class="card-title">🌍 Global Variables & Constants</div>
			<div style="max-height: 300px; overflow-y: auto;">
			<table>
				<thead>
					<tr>
						<th>Name</th>
						<th>Type</th>
						<th>Value</th>
						<th>File</th>
					</tr>
				</thead>
				<tbody>`)
	if len(globals) == 0 {
		mainDashboard.WriteString(`<tr><td colspan="4" style="text-align: center; color: var(--text-secondary);">No global declarations found.</td></tr>`)
	} else {
		for _, g := range globals {
			val := g.Value
			if val == "" {
				val = "—"
			}
			mainDashboard.WriteString(fmt.Sprintf(`
				<tr>
					<td style="font-family: monospace; font-weight: 600; color: var(--text-primary);">%s</td>
					<td style="font-family: monospace; color: var(--text-secondary);">%s</td>
					<td style="font-family: monospace; color: var(--accent-color); font-weight: 500;">%s</td>
					<td style="font-size: 0.85rem; font-family: monospace;">%s</td>
				</tr>`, html.EscapeString(g.Name), html.EscapeString(g.Type), html.EscapeString(val), html.EscapeString(g.File)))
		}
	}
	mainDashboard.WriteString(`</tbody></table></div></div></div>`)

	// Write index.html
	err := hg.renderPage(outputDir, "index.html", "Project Documentation Dashboard", sidebarRoot, mainDashboard.String(), 0)
	if err != nil {
		return err
	}

	// Write pages/search.html
	err = hg.generateSearchPage(source, outputDir, sidebarSub)
	if err != nil {
		return err
	}

	// Write pages/patterns.html
	err = hg.generatePatternsPage(source, outputDir, sidebarSub)
	if err != nil {
		return err
	}

	// Write pages/network.html
	err = hg.generateNetworkPage(source, outputDir, sidebarSub)
	if err != nil {
		return err
	}

	// ------------------ SUB-PAGES GENERATION ------------------

	// Group files by Package
	packageMap := make(map[string][]string)
	for _, f := range source.Files {
		pkgName := "main"
		for _, sym := range source.Symbols {
			if sym.File == f.Name && sym.Package != "" {
				pkgName = sym.Package
				break
			}
		}
		packageMap[pkgName] = append(packageMap[pkgName], f.Name)
	}

	// Generate Package Pages
	for pkg, files := range packageMap {
		var pkgBody strings.Builder
		pkgBody.WriteString(fmt.Sprintf(`<header>
            <h1>Package %s</h1>
            <p style="color: var(--text-secondary); margin-top: 0.5rem;">Details and components belonging to package %s.</p>
        </header>`, pkg, pkg))

		// Files Card
		pkgBody.WriteString(`<div class="card">
            <div class="card-title">📁 Registered Files</div>
            <ul style="margin: 0; padding-left: 1.2rem;">`)
		for _, f := range files {
			pkgBody.WriteString(fmt.Sprintf(`<li style="padding: 0.3rem 0; font-family: monospace; color: var(--text-secondary);">%s</li>`, f))
		}
		pkgBody.WriteString(`</ul></div>`)

		// ------------------ STRUCTS IN PACKAGE ------------------
		var pkgStructs []store.Symbol
		for _, s := range structs {
			if s.Package == pkg || (pkg == "main" && s.Package == "") {
				pkgStructs = append(pkgStructs, s)
			}
		}
		if len(pkgStructs) > 0 {
			pkgBody.WriteString(`<h2 style="margin: 2.5rem 0 1.2rem 0; color: var(--text-primary); font-size: 1.5rem; border-left: 4px solid var(--accent-color); padding-left: 0.6rem;">🧱 Structs</h2>`)
				for _, s := range pkgStructs {
					relationsText := ""
					if len(s.Relations) > 0 {
						relationsText = fmt.Sprintf(` <span style="font-size: 0.95rem; color: var(--text-secondary); font-weight: normal;">extends %s</span>`, renderTypeWithLinks(strings.Join(s.Relations, ", "), source))
					}
					sLang := getLanguageFromPath(s.File)
					pkgBody.WriteString(fmt.Sprintf(`
					<div class="card" id="struct_%s" data-lang="%s" style="scroll-margin-top: 2rem; margin-bottom: 2rem;">
						<div class="card-title" style="font-size: 1.3rem; border-bottom: 1px solid var(--border-color); padding-bottom: 0.5rem; margin-bottom: 1rem; color: var(--accent-color); display: flex; justify-content: space-between; align-items: center;">
							<span>struct %s%s</span>
							<span style="font-size: 0.85rem; color: var(--text-secondary); font-family: monospace; font-weight: normal;">%s:%d</span>
						</div>`, s.Name, sLang, s.Name, relationsText, s.File, s.Line))

				derived := source.GetDerivedSymbols(s.Name)
				if len(derived) > 0 {
					var links []string
					for _, d := range derived {
						dPkg := d.Package
						if dPkg == "" { dPkg = "main" }
						linkType := "struct"
						if d.Kind == store.SymInterface { linkType = "interface" }
						links = append(links, fmt.Sprintf(`<a href="pkg_%s.html#%s_%s" style="color: var(--accent-primary); text-decoration: underline; font-weight: 500;">%s</a>`, dPkg, linkType, d.Name, d.Name))
					}
					pkgBody.WriteString(fmt.Sprintf(`<div style="margin-bottom: 1.2rem; font-size: 0.9rem; color: var(--text-secondary);"><strong style="color: var(--text-primary);">Inherited by:</strong> %s</div>`, strings.Join(links, ", ")))
				}

				// Design Pattern Participation
				var matchedPatterns []string
				for _, p := range source.Patterns {
					for _, ps := range p.Symbols {
						if ps == s.Name {
							matchedPatterns = append(matchedPatterns, fmt.Sprintf(`<a href="patterns.html" style="display: inline-flex; align-items: center; background: rgba(255, 215, 0, 0.1); color: #ffd700; border: 1px solid rgba(255, 215, 0, 0.3); border-radius: 4px; padding: 2px 8px; font-size: 0.8rem; text-decoration: none; font-weight: 600; margin-right: 0.5rem;">🧩 Participates in %s</a>`, html.EscapeString(p.Name)))
							break
						}
					}
				}
				if len(matchedPatterns) > 0 {
					pkgBody.WriteString(fmt.Sprintf(`<div style="margin-bottom: 1.2rem; display: flex; flex-wrap: wrap; gap: 0.4rem;">%s</div>`, strings.Join(matchedPatterns, "")))
				}

				if s.MemorySize > 0 {
					pkgBody.WriteString(fmt.Sprintf(`<div style="margin-bottom: 1rem; display: flex; gap: 0.5rem; align-items: center;">
						<span style="display: inline-flex; align-items: center; background: rgba(16, 185, 129, 0.1); color: #10b981; border: 1px solid rgba(16, 185, 129, 0.3); border-radius: 4px; padding: 2px 8px; font-size: 0.8rem; font-weight: 600;">💾 Estimated Shallow Size: %d bytes</span>
					</div>`, s.MemorySize))
				}

				if s.Doc != "" {
					pkgBody.WriteString(fmt.Sprintf(`<div class="docblock" style="margin-bottom: 1.5rem; padding: 0.8rem 1rem; background: rgba(255,255,255,0.02); border-left: 3px solid var(--accent-color); border-radius: 0 4px 4px 0;">%s</div>`, renderMarkdownToHTML(s.Doc)))
				}

				// Fields Table
				fields := source.GetStructFields(s.Name)
				pkgBody.WriteString(`<h4 style="margin: 1rem 0 0.5rem 0; font-size: 1rem; color: var(--text-primary); font-weight: 600;">Fields</h4>
				<table style="margin-bottom: 1.5rem;">
					<thead><tr><th>Field</th><th>Type</th><th>Description</th></tr></thead>
					<tbody>`)
				if len(fields) == 0 {
					pkgBody.WriteString(`<tr><td colspan="3" style="text-align: center; color: var(--text-secondary);">No public fields declared.</td></tr>`)
				} else {
					for _, f := range fields {
						pkgBody.WriteString(fmt.Sprintf(`
							<tr>
								<td style="font-family: monospace; font-weight: 600; color: var(--text-primary);">%s</td>
								<td style="font-family: monospace; color: var(--text-secondary);">%s</td>
								<td style="font-size: 0.95rem;">%s</td>
							</tr>`, html.EscapeString(f.Name), renderTypeWithLinks(f.Type, source), html.EscapeString(f.Doc)))
					}
				}
				pkgBody.WriteString(`</tbody></table>`)

				// Receiver Methods Table
				methods := source.GetStructMethods(s.Name)
				pkgBody.WriteString(`<h4 style="margin: 1.5rem 0 0.5rem 0; font-size: 1rem; color: var(--text-primary); font-weight: 600;">Receiver Methods</h4>
				<table style="margin-bottom: 1.5rem;">
					<thead><tr><th>Method</th><th>Parameters</th><th>Returns</th><th>Coverage</th><th>CRAP</th></tr></thead>
					<tbody>`)
				if len(methods) == 0 {
					pkgBody.WriteString(`<tr><td colspan="5" style="text-align: center; color: var(--text-secondary);">No receiver methods implemented.</td></tr>`)
				} else {
					for _, m := range methods {
						covStr := "N/A"
						if m.Coverage != nil {
							covStr = fmt.Sprintf("%.1f%%", *m.Coverage)
						}
						pkgBody.WriteString(fmt.Sprintf(`
							<tr>
								<td style="font-family: monospace; font-weight: 600;"><a href="#func_%s_%s" style="color: var(--accent-primary); text-decoration: none;">%s</a></td>
								<td style="font-family: monospace; color: var(--text-secondary);">%s</td>
								<td style="font-family: monospace; color: var(--text-secondary);">%s</td>
								<td style="font-weight: 500;">%s</td>
								<td>%d</td>
							</tr>`, s.Name, m.Name, m.Name, m.Params, m.Returns, covStr, getCRAPScore(m)))
					}
				}
				pkgBody.WriteString(`</tbody></table>`)

				// Check for architectural diagrams
				timingImg := fmt.Sprintf("images/%s_timing.png", s.Name)
				timingImgPath := filepath.Join(outputDir, timingImg)
				_, errTiming := os.Stat(timingImgPath)

				typeImg := fmt.Sprintf("images/%s_type_graph.png", s.Name)
				typeImgPath := filepath.Join(outputDir, typeImg)
				_, errType := os.Stat(typeImgPath)

				if errTiming == nil || errType == nil {
					pkgBody.WriteString(`<h4 style="margin: 1.5rem 0 0.5rem 0; font-size: 1rem; color: var(--text-primary); font-weight: 600;">Architectural Diagrams</h4>
					<div style="display: grid; grid-template-columns: repeat(auto-fit, minmax(280px, 1fr)); gap: 1rem; margin-top: 0.5rem;">`)

					if errTiming == nil {
						pkgBody.WriteString(fmt.Sprintf(`
						<div style="text-align: center; border: 1px solid var(--border-color); border-radius: 8px; padding: 1rem; background: rgba(0,0,0,0.1);">
							<h5 style="margin-bottom: 0.5rem; font-size: 0.85rem; color: var(--text-secondary); font-weight: 600;">Struct Lifecycle Timing</h5>
							<a href="../graphs/%s_timing.html" target="_blank"><img src="../images/%s_timing.png" alt="Timing Diagram" style="max-width: 100%%; max-height: 250px; object-fit: contain; border-radius: 4px;"></a>
						</div>`, s.Name, s.Name))
					}

					if errType == nil {
						pkgBody.WriteString(fmt.Sprintf(`
						<div style="text-align: center; border: 1px solid var(--border-color); border-radius: 8px; padding: 1rem; background: rgba(0,0,0,0.1);">
							<h5 style="margin-bottom: 0.5rem; font-size: 0.85rem; color: var(--text-secondary); font-weight: 600;">Type Relations</h5>
							<a href="../graphs/%s_type.html" target="_blank"><img src="../images/%s_type_graph.png" alt="Type Graph" style="max-width: 100%%; max-height: 250px; object-fit: contain; border-radius: 4px;"></a>
						</div>`, s.Name, s.Name))
					}

					pkgBody.WriteString(`</div>`)
				}

				pkgBody.WriteString(`</div>`)
			}
		}

		// ------------------ INTERFACES IN PACKAGE ------------------
		var pkgInterfaces []store.Symbol
		for _, i := range interfaces {
			if i.Package == pkg || (pkg == "main" && i.Package == "") {
				pkgInterfaces = append(pkgInterfaces, i)
			}
		}
		if len(pkgInterfaces) > 0 {
			pkgBody.WriteString(`<h2 style="margin: 2.5rem 0 1.2rem 0; color: var(--text-primary); font-size: 1.5rem; border-left: 4px solid var(--accent-color); padding-left: 0.6rem;">🔌 Interfaces</h2>`)
			for _, i := range pkgInterfaces {
				relationsText := ""
				if len(i.Relations) > 0 {
					relationsText = fmt.Sprintf(` <span style="font-size: 0.95rem; color: var(--text-secondary); font-weight: normal;">extends %s</span>`, renderTypeWithLinks(strings.Join(i.Relations, ", "), source))
				}
				iLang := getLanguageFromPath(i.File)
				pkgBody.WriteString(fmt.Sprintf(`
				<div class="card" id="interface_%s" data-lang="%s" style="scroll-margin-top: 2rem; margin-bottom: 2rem;">
					<div class="card-title" style="font-size: 1.3rem; border-bottom: 1px solid var(--border-color); padding-bottom: 0.5rem; margin-bottom: 1rem; color: var(--accent-color); display: flex; justify-content: space-between; align-items: center;">
						<span>interface %s%s</span>
						<span style="font-size: 0.85rem; color: var(--text-secondary); font-family: monospace; font-weight: normal;">%s:%d</span>
					</div>`, i.Name, iLang, i.Name, relationsText, i.File, i.Line))

				derived := source.GetDerivedSymbols(i.Name)
				if len(derived) > 0 {
					var links []string
					for _, d := range derived {
						dPkg := d.Package
						if dPkg == "" { dPkg = "main" }
						linkType := "struct"
						if d.Kind == store.SymInterface { linkType = "interface" }
						links = append(links, fmt.Sprintf(`<a href="pkg_%s.html#%s_%s" style="color: var(--accent-primary); text-decoration: underline; font-weight: 500;">%s</a>`, dPkg, linkType, d.Name, d.Name))
					}
					pkgBody.WriteString(fmt.Sprintf(`<div style="margin-bottom: 1.2rem; font-size: 0.9rem; color: var(--text-secondary);"><strong style="color: var(--text-primary);">Inherited by:</strong> %s</div>`, strings.Join(links, ", ")))
				}

				// Design Pattern Participation
				var matchedPatI []string
				for _, p := range source.Patterns {
					for _, ps := range p.Symbols {
						if ps == i.Name {
							matchedPatI = append(matchedPatI, fmt.Sprintf(`<a href="patterns.html" style="display: inline-flex; align-items: center; background: rgba(255, 215, 0, 0.1); color: #ffd700; border: 1px solid rgba(255, 215, 0, 0.3); border-radius: 4px; padding: 2px 8px; font-size: 0.8rem; text-decoration: none; font-weight: 600; margin-right: 0.5rem;">🧩 Participates in %s</a>`, html.EscapeString(p.Name)))
							break
						}
					}
				}
				if len(matchedPatI) > 0 {
					pkgBody.WriteString(fmt.Sprintf(`<div style="margin-bottom: 1.2rem; display: flex; flex-wrap: wrap; gap: 0.4rem;">%s</div>`, strings.Join(matchedPatI, "")))
				}

				if i.Doc != "" {
					pkgBody.WriteString(fmt.Sprintf(`<div class="docblock" style="padding: 0.8rem 1rem; background: rgba(255,255,255,0.02); border-left: 3px solid var(--accent-color); border-radius: 0 4px 4px 0;">%s</div>`, renderMarkdownToHTML(i.Doc)))
				}
				pkgBody.WriteString(`</div>`)
			}
		}

		// ------------------ FUNCTIONS & METHODS IN PACKAGE ------------------
		var pkgFuncs []store.Symbol
		for _, f := range funcs {
			if f.Package == pkg || (pkg == "main" && f.Package == "") {
				pkgFuncs = append(pkgFuncs, f)
			}
		}
		allMethods := getSymbolsOfKind(source, store.SymMethod)
		for _, m := range allMethods {
			if m.Package == pkg || (pkg == "main" && m.Package == "") {
				pkgFuncs = append(pkgFuncs, m)
			}
		}

		if len(pkgFuncs) > 0 {
			pkgBody.WriteString(`<h2 style="margin: 2.5rem 0 1.2rem 0; color: var(--text-primary); font-size: 1.5rem; border-left: 4px solid var(--accent-color); padding-left: 0.6rem;">λ Functions & Methods</h2>`)
			for _, f := range pkgFuncs {
				covStr := "N/A"
				if f.Coverage != nil {
					covStr = fmt.Sprintf("%.1f%%", *f.Coverage)
				}

				cardID := "func_" + f.Name
				displayName := "func " + f.Name + "()"
				sigName := f.Name
				if f.Parent != "" {
					cardID = "func_" + f.Parent + "_" + f.Name
					displayName = fmt.Sprintf("func (%s) %s()", f.Parent, f.Name)
					sigName = fmt.Sprintf("(r *%s) %s", f.Parent, f.Name)
				}

				asyncBadge := ""
				if f.IsAsync {
					asyncBadge = ` <span style="background: linear-gradient(135deg, #6a11cb 0%, #2575fc 100%); color: white; font-size: 0.65rem; text-transform: uppercase; padding: 3px 8px; border-radius: 20px; margin-left: 8px; vertical-align: middle; font-weight: bold; letter-spacing: 0.5px; box-shadow: 0 2px 6px rgba(0,0,0,0.3);">Async</span>`
				}
				if f.SpawnsThread {
					asyncBadge += ` <span style="background: linear-gradient(135deg, #ef4444 0%, #f59e0b 100%); color: white; font-size: 0.65rem; text-transform: uppercase; padding: 3px 8px; border-radius: 20px; margin-left: 8px; vertical-align: middle; font-weight: bold; letter-spacing: 0.5px; box-shadow: 0 2px 6px rgba(0,0,0,0.3);">🧵 Spawns Thread</span>`
				}

				fLang := getLanguageFromPath(f.File)
				pkgBody.WriteString(fmt.Sprintf(`
				<div class="card" id="%s" data-lang="%s" style="scroll-margin-top: 2rem; margin-bottom: 2rem;">
					<div class="card-title" style="font-size: 1.3rem; border-bottom: 1px solid var(--border-color); padding-bottom: 0.5rem; margin-bottom: 1rem; color: var(--accent-color); display: flex; justify-content: space-between; align-items: center;">
						<span style="font-family: monospace; font-weight: 600; display: flex; align-items: center;">%s%s</span>
						<span style="font-size: 0.85rem; color: var(--text-secondary); font-family: monospace; font-weight: normal;">%s:%d</span>
					</div>`, cardID, fLang, displayName, asyncBadge, f.File, f.Line))

				if f.Doc != "" {
					pkgBody.WriteString(fmt.Sprintf(`<div class="docblock" style="margin-bottom: 1.5rem; padding: 0.8rem 1rem; background: rgba(255,255,255,0.02); border-left: 3px solid var(--accent-color); border-radius: 0 4px 4px 0;">%s</div>`, renderMarkdownToHTML(f.Doc)))
				}

				// Metrics
				pkgBody.WriteString(fmt.Sprintf(`
				<div style="display: grid; grid-template-columns: repeat(auto-fit, minmax(130px, 1fr)); gap: 1rem; margin-bottom: 1.5rem;">
					<div style="border: 1px solid var(--border-color); padding: 0.6rem 0.8rem; border-radius: 6px; text-align: center; background: rgba(255,255,255,0.01);">
						<div style="font-size: 0.75rem; color: var(--text-secondary); font-weight: 600; letter-spacing: 0.05em;">LINE COUNT</div>
						<div style="font-size: 1.25rem; font-weight: 700; color: var(--text-primary); margin-top: 0.2rem;">%d</div>
					</div>
					<div style="border: 1px solid var(--border-color); padding: 0.6rem 0.8rem; border-radius: 6px; text-align: center; background: rgba(255,255,255,0.01);">
						<div style="font-size: 0.75rem; color: var(--text-secondary); font-weight: 600; letter-spacing: 0.05em;">COMPLEXITY</div>
						<div style="font-size: 1.25rem; font-weight: 700; color: var(--text-primary); margin-top: 0.2rem;">%d</div>
					</div>
					<div style="border: 1px solid var(--border-color); padding: 0.6rem 0.8rem; border-radius: 6px; text-align: center; background: rgba(255,255,255,0.01);">
						<div style="font-size: 0.75rem; color: var(--text-secondary); font-weight: 600; letter-spacing: 0.05em;">COVERAGE</div>
						<div style="font-size: 1.25rem; font-weight: 700; color: var(--text-primary); margin-top: 0.2rem;">%s</div>
					</div>
					<div style="border: 1px solid var(--border-color); padding: 0.6rem 0.8rem; border-radius: 6px; text-align: center; background: rgba(255,255,255,0.01);">
						<div style="font-size: 0.75rem; color: var(--text-secondary); font-weight: 600; letter-spacing: 0.05em;">CRAP INDEX</div>
						<div style="font-size: 1.25rem; font-weight: 700; color: var(--text-primary); margin-top: 0.2rem;">%d</div>
					</div>
				</div>`, f.LineCount, f.Complexity, covStr, getCRAPScore(f)))

				// Signature
				pkgBody.WriteString(fmt.Sprintf(`
				<h5 style="margin: 0 0 0.4rem 0; font-size: 0.9rem; color: var(--text-secondary); font-weight: 600;">λ Signature</h5>
				<pre style="margin-bottom: 1.5rem;"><code class="language-go">func %s%s %s</code></pre>`, html.EscapeString(sigName), html.EscapeString(f.Params), html.EscapeString(f.Returns)))

				// Compute function call graph key
				funcKey := f.Name
				if f.Parent != "" {
					if f.Package != "" {
						funcKey = f.Package + "." + f.Parent + "." + f.Name
					} else {
						funcKey = f.Parent + "." + f.Name
					}
				} else if f.Package != "" {
					funcKey = f.Package + "." + f.Name
				}
				cleanFuncKey := strings.ReplaceAll(funcKey, ".", "_")

				callImg := fmt.Sprintf("images/%s_call_graph.png", cleanFuncKey)
				callImgPath := filepath.Join(outputDir, callImg)
				if _, errCall := os.Stat(callImgPath); errCall == nil {
					pkgBody.WriteString(fmt.Sprintf(`
					<h5 style="margin: 0 0 0.4rem 0; font-size: 0.9rem; color: var(--text-secondary); font-weight: 600;">Call Graph Diagram</h5>
					<div style="text-align: center; border: 1px solid var(--border-color); border-radius: 8px; padding: 1rem; background: rgba(0,0,0,0.1); margin-top: 0.5rem; margin-bottom: 1.5rem;">
						<a href="../graphs/%s_call.html" target="_blank"><img src="../images/%s_call_graph.png" alt="Call Graph" style="max-width: 100%%; max-height: 250px; object-fit: contain; border-radius: 4px;"></a>
					</div>`, cleanFuncKey, cleanFuncKey))
				}

				seqImg := fmt.Sprintf("images/%s_sequence.png", cleanFuncKey)
				seqImgPath := filepath.Join(outputDir, seqImg)
				if _, errSeq := os.Stat(seqImgPath); errSeq == nil {
					pkgBody.WriteString(fmt.Sprintf(`
					<h5 style="margin: 0 0 0.4rem 0; font-size: 0.9rem; color: var(--text-secondary); font-weight: 600;">Sequence Diagram</h5>
					<div style="text-align: center; border: 1px solid var(--border-color); border-radius: 8px; padding: 1rem; background: rgba(0,0,0,0.1); margin-top: 0.5rem; margin-bottom: 1.5rem;">
						<a href="../graphs/%s_sequence.html" target="_blank"><img src="../images/%s_sequence.png" alt="Sequence Diagram" style="max-width: 100%%; max-height: 250px; object-fit: contain; border-radius: 4px;"></a>
					</div>`, cleanFuncKey, cleanFuncKey))
				}

				pkgBody.WriteString(`</div>`)
			}
		}

		filename := fmt.Sprintf("pages/pkg_%s.html", pkg)
		_ = hg.renderPage(outputDir, filename, "Package "+pkg, sidebarSub, pkgBody.String(), 1)
	}

	return nil
}

// renderTypeWithLinks wraps matching struct types with links
func renderTypeWithLinks(typeStr string, source *store.Source) string {
	if typeStr == "" {
		return ""
	}
	escaped := html.EscapeString(typeStr)

	structs := getSymbolsOfKind(source, store.SymStruct)
	interfaces := getSymbolsOfKind(source, store.SymInterface)

	words := strings.Fields(escaped)
	for i, word := range words {
		cleanWord := word
		cleanWord = strings.Trim(cleanWord, "*[],.()")
		if idx := strings.LastIndex(cleanWord, "."); idx != -1 {
			cleanWord = cleanWord[idx+1:]
		}
		matched := false
		for _, s := range structs {
			if cleanWord == s.Name {
				pkgName := s.Package
				if pkgName == "" {
					pkgName = "main"
				}
				linked := fmt.Sprintf(`<a href="pkg_%s.html#struct_%s" style="color: #818CF8; text-decoration: underline;">%s</a>`, pkgName, s.Name, s.Name)
				words[i] = strings.Replace(word, s.Name, linked, 1)
				matched = true
				break
			}
		}
		if !matched {
			for _, s := range interfaces {
				if cleanWord == s.Name {
					pkgName := s.Package
					if pkgName == "" {
						pkgName = "main"
					}
					linked := fmt.Sprintf(`<a href="pkg_%s.html#interface_%s" style="color: #34D399; text-decoration: underline;">%s</a>`, pkgName, s.Name, s.Name)
					words[i] = strings.Replace(word, s.Name, linked, 1)
					break
				}
			}
		}
	}
	return strings.Join(words, " ")
}

// Inline markdown compiler
func renderMarkdownToHTML(md string) string {
	var html strings.Builder
	lines := strings.Split(md, "\n")
	inCodeBlock := false
	inList := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		if strings.HasPrefix(trimmed, "```") {
			if inCodeBlock {
				html.WriteString("</code></pre>\n")
				inCodeBlock = false
			} else {
				lang := strings.TrimPrefix(trimmed, "```")
				if lang == "" {
					lang = "text"
				}
				html.WriteString(fmt.Sprintf("<pre><code class=\"language-%s\">", lang))
				inCodeBlock = true
			}
			continue
		}

		if inCodeBlock {
			html.WriteString(strings.ReplaceAll(strings.ReplaceAll(line, "&", "&amp;"), "<", "&lt;") + "\n")
			continue
		}

		tagRegex := regexp.MustCompile(`^[@/\\](param|return|returns|brief|note|warning|deprecated|see)\b\s*(.*)$`)
		if m := tagRegex.FindStringSubmatch(trimmed); m != nil {
			cmd := strings.ToLower(m[1])
			remainder := strings.TrimSpace(m[2])

			switch cmd {
			case "param":
				parts := strings.SplitN(remainder, " ", 2)
				var name, desc string
				if len(parts) > 0 {
					name = parts[0]
				}
				if len(parts) > 1 {
					desc = parts[1]
				}
				html.WriteString(fmt.Sprintf(`<div class="tag-param" style="margin: 0.4rem 0; padding-left: 0.6rem; border-left: 3px solid var(--accent-color);"><strong style="color: var(--text-primary);">Parameter</strong> <code style="color: var(--accent-color); font-weight: 600;">%s</code> — %s</div>`+"\n", name, parseInlineMarkdown(desc)))
			case "return", "returns":
				html.WriteString(fmt.Sprintf(`<div class="tag-return" style="margin: 0.4rem 0; padding-left: 0.6rem; border-left: 3px solid #10b981;"><strong style="color: #10b981;">Returns:</strong> %s</div>`+"\n", parseInlineMarkdown(remainder)))
			case "brief":
				html.WriteString(fmt.Sprintf(`<p style="font-size: 1.1rem; font-weight: 500; color: var(--text-primary); margin-bottom: 0.5rem;">%s</p>`+"\n", parseInlineMarkdown(remainder)))
			case "note":
				html.WriteString(fmt.Sprintf(`<div class="callout-note" style="margin: 0.8rem 0; padding: 0.6rem 0.8rem; background: rgba(59, 130, 246, 0.08); border-left: 4px solid #3b82f6; border-radius: 4px;"><strong style="color: #3b82f6;">ℹ️ Note:</strong> %s</div>`+"\n", parseInlineMarkdown(remainder)))
			case "warning":
				html.WriteString(fmt.Sprintf(`<div class="callout-warning" style="margin: 0.8rem 0; padding: 0.6rem 0.8rem; background: rgba(245, 158, 11, 0.08); border-left: 4px solid #f59e0b; border-radius: 4px;"><strong style="color: #f59e0b;">⚠️ Warning:</strong> %s</div>`+"\n", parseInlineMarkdown(remainder)))
			case "deprecated":
				html.WriteString(fmt.Sprintf(`<div class="callout-danger" style="margin: 0.8rem 0; padding: 0.6rem 0.8rem; background: rgba(239, 68, 68, 0.08); border-left: 4px solid #ef4444; border-radius: 4px;"><strong style="color: #ef4444;">🚫 Deprecated:</strong> %s</div>`+"\n", parseInlineMarkdown(remainder)))
			case "see":
				html.WriteString(fmt.Sprintf(`<div class="tag-see" style="margin: 0.4rem 0; color: var(--text-secondary);"><strong>See also:</strong> <code style="color: var(--accent-color); font-weight: 600;">%s</code></div>`+"\n", parseInlineMarkdown(remainder)))
			}
			continue
		}

		if strings.HasPrefix(trimmed, "- ") || strings.HasPrefix(trimmed, "* ") {
			if !inList {
				html.WriteString("<ul>\n")
				inList = true
			}
			itemContent := strings.TrimPrefix(strings.TrimPrefix(trimmed, "- "), "* ")
			html.WriteString(fmt.Sprintf("<li>%s</li>\n", parseInlineMarkdown(itemContent)))
			continue
		} else if inList && trimmed == "" {
			html.WriteString("</ul>\n")
			inList = false
		}

		if strings.HasPrefix(trimmed, "# ") {
			html.WriteString(fmt.Sprintf("<h1>%s</h1>\n", parseInlineMarkdown(strings.TrimPrefix(trimmed, "# "))))
			continue
		} else if strings.HasPrefix(trimmed, "## ") {
			html.WriteString(fmt.Sprintf("<h2>%s</h2>\n", parseInlineMarkdown(strings.TrimPrefix(trimmed, "## "))))
			continue
		} else if strings.HasPrefix(trimmed, "### ") {
			html.WriteString(fmt.Sprintf("<h3>%s</h3>\n", parseInlineMarkdown(strings.TrimPrefix(trimmed, "### "))))
			continue
		}

		if trimmed != "" {
			html.WriteString(fmt.Sprintf("<p>%s</p>\n", parseInlineMarkdown(trimmed)))
		} else {
			html.WriteString("<br>\n")
		}
	}

	if inList {
		html.WriteString("</ul>\n")
	}

	return html.String()
}

func parseInlineMarkdown(text string) string {
	text = strings.ReplaceAll(text, "&", "&amp;")
	text = strings.ReplaceAll(text, "<", "&lt;")
	text = strings.ReplaceAll(text, ">", "&gt;")

	for {
		start := strings.Index(text, "`")
		if start == -1 {
			break
		}
		end := strings.Index(text[start+1:], "`")
		if end == -1 {
			break
		}
		end = start + 1 + end
		codeContent := text[start+1 : end]
		text = text[:start] + fmt.Sprintf("<code>%s</code>", codeContent) + text[end+1:]
	}

	for {
		start := strings.Index(text, "**")
		if start == -1 {
			break
		}
		end := strings.Index(text[start+2:], "**")
		if end == -1 {
			break
		}
		end = start + 2 + end
		boldContent := text[start+2 : end]
		text = text[:start] + fmt.Sprintf("<strong>%s</strong>", boldContent) + text[end+2:]
	}

	for {
		start := strings.Index(text, "[")
		if start == -1 {
			break
		}
		mid := strings.Index(text[start:], "](")
		if mid == -1 {
			break
		}
		mid = start + mid
		end := strings.Index(text[mid:], ")")
		if end == -1 {
			break
		}
		end = mid + end
		linkText := text[start+1 : mid]
		linkURL := text[mid+2 : end]
		text = text[:start] + fmt.Sprintf("<a href=\"%s\" style=\"color: var(--accent-primary); text-decoration: underline;\">%s</a>", linkURL, linkText) + text[end+1:]
	}

	return text
}

// getLanguageFromPath determines the display name of the language based on file extension.
func getLanguageFromPath(filePath string) string {
	ext := strings.ToLower(filepath.Ext(filePath))
	switch ext {
	case ".go":
		return "Go"
	case ".py":
		return "Python"
	case ".kt", ".kts":
		return "Kotlin"
	case ".java":
		return "Java"
	case ".odin":
		return "Odin"
	case ".js", ".ts":
		return "JavaScript"
	case ".md":
		return "Markdown"
	default:
		return "Other"
	}
}

type searchItem struct {
	Name     string `json:"n"`
	Kind     string `json:"k"`
	Package  string `json:"p"`
	File     string `json:"f"`
	Line     int    `json:"l"`
	Link     string `json:"u"`
	Language string `json:"lang"`
}

func (hg *HTMLGenerator) generateSearchPage(source *store.Source, outputDir string, sidebar string) error {
	var items []searchItem
	for _, s := range source.Symbols {
		if s.Kind == store.SymImport || s.Kind == store.SymField {
			continue // Exclude noisy items
		}
		
		pkg := s.Package
		if pkg == "" { pkg = "main" }
		
		linkType := "struct"
		if s.Kind == store.SymInterface {
			linkType = "interface"
		} else if s.Kind == store.SymFunction || s.Kind == store.SymMethod {
			linkType = "func"
		}
		
		page := "pkg_" + pkg + ".html"
		anchor := "#" + linkType + "_" + s.Name
		if s.Kind == store.SymMethod && s.Parent != "" {
			anchor = "#func_" + s.Parent + "_" + s.Name
		}
		
		items = append(items, searchItem{
			Name:     s.Name,
			Kind:     string(s.Kind),
			Package:  pkg,
			File:     s.File,
			Line:     s.Line,
			Link:     page + anchor,
			Language: getLanguageFromPath(s.File),
		})
	}

	jsonData, _ := json.Marshal(items)

	var body strings.Builder
	body.WriteString(`
    <header>
        <h1>🔍 Full Symbol Search</h1>
        <p style="color: var(--text-secondary); margin-top: 0.5rem;">Instantly search through all symbols in the project database.</p>
    </header>
    
    <div class="card">
        <input type="text" id="searchInput" placeholder="Type to filter by symbol name, package, or language..." style="width: 100%; padding: 1rem 1.2rem; font-size: 1.1rem; background: rgba(0,0,0,0.2); border: 1px solid var(--border-color); color: var(--text-primary); border-radius: 8px; font-family: 'Inter', sans-serif; outline: none; box-shadow: 0 4px 12px rgba(0,0,0,0.15); transition: border-color 0.2s;" onfocus="this.style.borderColor='var(--accent-primary)'" onblur="this.style.borderColor='var(--border-color)'">
    </div>

    <div class="card" style="padding: 0; overflow: hidden;">
        <div style="max-height: 70vh; overflow-y: auto;">
            <table style="margin-bottom: 0; width: 100%; border-collapse: collapse;">
                <thead style="position: sticky; top: 0; background: var(--sidebar-bg); z-index: 5;">
                    <tr>
                        <th style="padding: 1rem;">Symbol Name</th>
                        <th style="padding: 1rem;">Kind</th>
                        <th style="padding: 1rem;">Package</th>
                        <th style="padding: 1rem;">Language</th>
                    </tr>
                </thead>
                <tbody id="searchResults">
                    <!-- Dynamically Populated -->
                </tbody>
            </table>
        </div>
    </div>
    <div id="resultCount" style="margin-top: 1rem; font-size: 0.85rem; color: var(--text-secondary); text-align: right;"></div>

    <script>
        const symbols = ` + string(jsonData) + `;
        const input = document.getElementById('searchInput');
        const results = document.getElementById('searchResults');
        const count = document.getElementById('resultCount');

        function performSearch() {
            const q = input.value.toLowerCase().trim();
            
            // Filter and cap at 500 items for UI performance
            const matched = symbols.filter(s => {
                return s.n.toLowerCase().includes(q) || 
                       s.p.toLowerCase().includes(q) || 
                       s.lang.toLowerCase().includes(q) ||
                       s.k.toLowerCase().includes(q);
            }).slice(0, 500);

            let html = "";
            if (matched.length === 0) {
                html = '<tr><td colspan="4" style="text-align: center; padding: 3rem; color: var(--text-secondary); font-style: italic;">No matching symbols found.</td></tr>';
            } else {
                matched.forEach(s => {
                    let kindEmoji = "🏷️";
                    if(s.k === "struct") kindEmoji = "🧱";
                    else if(s.k === "interface") kindEmoji = "🔌";
                    else if(s.k === "function" || s.k === "method") kindEmoji = "λ";
                    
                    html += '<tr data-lang="' + s.lang + '">' +
                        '<td style="padding: 0.75rem 1rem;"><a href="' + s.u + '" style="color: var(--accent-primary); font-family: monospace; font-weight: 600; text-decoration: none; display: block;">' + kindEmoji + ' ' + s.n + '</a></td>' +
                        '<td style="padding: 0.75rem 1rem; text-transform: capitalize; color: var(--text-secondary); font-size: 0.9rem;">' + s.k + '</td>' +
                        '<td style="padding: 0.75rem 1rem; color: var(--text-primary); font-size: 0.9rem; font-family: monospace;">' + s.p + '</td>' +
                        '<td style="padding: 0.75rem 1rem;"><span style="font-size: 0.75rem; background: rgba(255,255,255,0.1); padding: 0.2rem 0.5rem; border-radius: 4px;">' + s.lang + '</span></td>' +
                    '</tr>';
                });
            }
            results.innerHTML = html;
            count.innerText = 'Showing ' + matched.length + ' symbols';
            
            if(window.applyFilters) window.applyFilters();
        }

        input.addEventListener('input', performSearch);
        window.addEventListener('DOMContentLoaded', performSearch);
    </script>
    `)

	return hg.renderPage(outputDir, filepath.Join("pages", "search.html"), "Symbol Search", sidebar, body.String(), 1)
}

func (hg *HTMLGenerator) generatePatternsPage(source *store.Source, outputDir string, sidebar string) error {
	var page strings.Builder
	page.WriteString(`<header>
        <h1>🧩 Design Architecture Patterns</h1>
        <p style="color: var(--text-secondary); margin-top: 0.5rem;">Automatically detected architectural and creational design patterns implemented across the project codebase.</p>
    </header>`)

	if len(source.Patterns) == 0 {
		page.WriteString(`<div class="card" style="text-align: center; padding: 4rem 2rem;">
            <div style="font-size: 3rem; margin-bottom: 1rem;">🔍</div>
            <h3 style="color: var(--text-secondary);">No distinct GoF or Game Patterns were programmatically identified.</h3>
        </div>`)
	} else {
		page.WriteString(`<div style="display: grid; grid-template-columns: 1fr; gap: 2rem; margin-top: 2rem;">`)
		for i, p := range source.Patterns {
			page.WriteString(fmt.Sprintf(`<div class="card" style="border-left: 5px solid var(--accent-primary);">
                <div style="display: flex; justify-content: space-between; align-items: flex-start; margin-bottom: 1rem;">
                    <div>
                        <span class="tag" style="background: var(--tag-blue-bg); color: var(--tag-blue-text); margin-bottom: 0.5rem; display: inline-block;">%s</span>
                        <h2 style="margin: 0; font-size: 1.4rem;">%s</h2>
                    </div>
                </div>
                <p style="color: var(--text-secondary); font-size: 0.95rem; margin-bottom: 1.5rem;">%s</p>

                <h4 style="margin-bottom: 0.5rem;">👥 Participating Symbols</h4>
                <div style="display: flex; flex-wrap: wrap; gap: 0.5rem; margin-bottom: 1.5rem;">`, 
                html.EscapeString(p.Category), html.EscapeString(p.Name), html.EscapeString(p.Description)))
			
			for _, sym := range p.Symbols {
				// Find symbol package to construct accurate local link
				targetPkg := "main"
				for _, s := range source.Symbols {
					if s.Name == sym && s.Package != "" {
						targetPkg = s.Package
						break
					}
				}
				page.WriteString(fmt.Sprintf(`<a href="%s.html#%s" style="background: var(--bg-secondary); border: 1px solid var(--border-color); padding: 0.25rem 0.75rem; border-radius: 4px; color: var(--accent-primary); text-decoration: none; font-family: monospace; font-size: 0.85rem;">%s</a>`, 
                    html.EscapeString(targetPkg), html.EscapeString(sym), html.EscapeString(sym)))
			}
			page.WriteString(`</div>`)

			// Embed Pattern diagram
			imgPath := fmt.Sprintf("../images/pattern_%d.png", i)
			// NOTE: In local FS context from `Generate` we evaluate physical location to check if file exists
			imgFile := filepath.Join(outputDir, "images", fmt.Sprintf("pattern_%d.png", i))
			if _, err := os.Stat(imgFile); err == nil {
				page.WriteString(fmt.Sprintf(`
				<div style="background: white; border: 1px solid var(--border-color); border-radius: 8px; padding: 1rem; text-align: center; margin-top: 1rem;">
					<img src="%s" alt="%s diagram" style="max-width: 100%%; height: auto;" loading="lazy">
				</div>`, imgPath, html.EscapeString(p.Name)))
			}
			page.WriteString(`</div>`)
		}
		page.WriteString(`</div>`)
	}

	return hg.renderPage(outputDir, filepath.Join("pages", "patterns.html"), "Design Patterns", sidebar, page.String(), 1)
}

func (hg *HTMLGenerator) generateNetworkPage(source *store.Source, outputDir string, sidebar string) error {
	var page strings.Builder
	page.WriteString(`<header>
        <h1>🌐 Network Architecture & Audit</h1>
        <p style="color: var(--text-secondary); margin-top: 0.5rem;">Audit report analyzing edge services, transport protocols, latency mitigations, and game netcode models across source nodes.</p>
    </header>`)

	if len(source.NetworkAnalysis) == 0 {
		page.WriteString(`<div class="card" style="text-align: center; padding: 4rem 2rem;">
            <div style="font-size: 3rem; margin-bottom: 1rem;">📡</div>
            <h3 style="color: var(--text-secondary);">No specific network transport, edge server, or latency mitigation artifacts detected.</h3>
        </div>`)
	} else {
		page.WriteString(`<div style="display: grid; grid-template-columns: 1fr; gap: 2rem; margin-top: 2rem;">`)
		for i, nc := range source.NetworkAnalysis {
			
			// Color code specific types
			tagColor := "var(--tag-purple-bg)"
			txtColor := "var(--tag-purple-text)"
			if strings.Contains(nc.Type, "Mitigation") {
				tagColor = "var(--tag-green-bg)"
				txtColor = "var(--tag-green-text)"
			} else if strings.Contains(nc.Type, "Transport") {
				tagColor = "var(--tag-blue-bg)"
				txtColor = "var(--tag-blue-text)"
			} else if strings.Contains(nc.Type, "Realtime") {
				tagColor = "rgba(255, 152, 0, 0.15)"
				txtColor = "#E67E22"
			} else if strings.Contains(nc.Type, "Security") {
				tagColor = "rgba(231, 76, 60, 0.15)"
				txtColor = "#C0392B"
			} else if strings.Contains(nc.Type, "Traffic") {
				tagColor = "rgba(52, 152, 219, 0.15)"
				txtColor = "#2980B9"
			} else if strings.Contains(nc.Type, "Caching") {
				tagColor = "rgba(46, 204, 113, 0.15)"
				txtColor = "#27AE60"
			}

			page.WriteString(fmt.Sprintf(`<div class="card" style="border-left: 5px solid #3498DB; position: relative; overflow: hidden;">
				<div style="position: absolute; right: -20px; top: -20px; font-size: 8rem; opacity: 0.03; font-weight: bold; pointer-events: none;">NET</div>
                <div style="display: flex; justify-content: space-between; align-items: flex-start; margin-bottom: 1rem;">
                    <div>
                        <span class="tag" style="background: %s; color: %s; margin-bottom: 0.5rem; display: inline-block;">%s</span>
                        <h2 style="margin: 0; font-size: 1.4rem; color: var(--text-primary);">%s</h2>
                    </div>
                </div>
                <p style="color: var(--text-secondary); font-size: 0.95rem; margin-bottom: 1.5rem;">%s</p>`, 
                tagColor, txtColor, html.EscapeString(nc.Type), html.EscapeString(nc.Name), html.EscapeString(nc.Description)))

			// Show explicit details table if available
			if len(nc.Details) > 0 {
				page.WriteString(`<table style="width: 100%; border-collapse: collapse; margin-bottom: 1.5rem; font-size: 0.85rem; background: rgba(0,0,0,0.02); border-radius: 4px;">`)
				for k, v := range nc.Details {
					page.WriteString(fmt.Sprintf(`
					<tr style="border-bottom: 1px solid var(--border-color);">
						<td style="padding: 0.5rem; font-weight: bold; width: 30%%; color: var(--text-secondary);">%s</td>
						<td style="padding: 0.5rem; font-family: monospace; color: var(--text-primary);">%s</td>
					</tr>`, html.EscapeString(k), html.EscapeString(v)))
				}
				page.WriteString(`</table>`)
			}

			page.WriteString(`<h4 style="margin-bottom: 0.5rem; font-size: 0.9rem; color: var(--text-secondary);">📦 Anchored Source Nodes</h4>
                <div style="display: flex; flex-wrap: wrap; gap: 0.5rem; margin-bottom: 1.5rem;">`)
			for _, sym := range nc.Symbols {
				targetPkg := "main"
				for _, s := range source.Symbols {
					if s.Name == sym && s.Package != "" {
						targetPkg = s.Package
						break
					}
				}
				page.WriteString(fmt.Sprintf(`<a href="pkg_%s.html#%s" style="background: var(--bg-secondary); border: 1px solid var(--border-color); padding: 0.25rem 0.75rem; border-radius: 4px; color: var(--accent-primary); text-decoration: none; font-family: monospace; font-size: 0.85rem; z-index: 2;">%s</a>`, 
                    html.EscapeString(targetPkg), html.EscapeString(sym), html.EscapeString(sym)))
			}
			page.WriteString(`</div>`)

			// Render explicit connection nodes (Top 5 callers/callees)
			if len(nc.Symbols) > 0 {
				mainSym := nc.Symbols[0]
				callers := source.GetCallers(mainSym)
				if len(callers) > 0 {
					page.WriteString(`<div style="margin-bottom: 1rem;"><span style="font-size: 0.8rem; color: var(--text-secondary); font-weight: bold;">⬇️ Top Inbound Connectors: </span><div style="display: flex; flex-wrap: wrap; gap: 0.25rem; margin-top: 0.25rem;">`)
					limit := 5
					if len(callers) < 5 { limit = len(callers) }
					for _, c := range callers[:limit] {
						page.WriteString(fmt.Sprintf(`<span style="background: rgba(0,0,0,0.03); font-size: 0.75rem; padding: 0.2rem 0.5rem; border-radius: 3px; border: 1px dotted #ccc; font-family: monospace;">%s</span>`, html.EscapeString(c)))
					}
					if len(callers) > 5 { page.WriteString(`<span style="font-size: 0.75rem; color: var(--text-secondary);">...</span>`) }
					page.WriteString(`</div></div>`)
				}
			}

			// Embed Generated Network Graph
			imgID := fmt.Sprintf("network_%d", i)
			imgRelPath := fmt.Sprintf("../images/%s.png", imgID)
			imgFile := filepath.Join(outputDir, "images", imgID+".png")
			if _, err := os.Stat(imgFile); err == nil {
				page.WriteString(fmt.Sprintf(`
				<div style="background: #fdfdfd; border: 1px solid var(--border-color); border-radius: 8px; padding: 1.5rem; text-align: center; margin-top: 1rem; box-shadow: inset 0 0 10px rgba(0,0,0,0.02);">
					<div style="font-size: 0.75rem; text-transform: uppercase; color: var(--text-secondary); margin-bottom: 0.75rem; font-weight: bold;">Architecture Topology</div>
					<img src="%s" alt="%s architecture" style="max-width: 100%%; height: auto; filter: drop-shadow(0 4px 6px rgba(0,0,0,0.05));" loading="lazy">
				</div>`, imgRelPath, html.EscapeString(nc.Name)))
			}
			page.WriteString(`</div>`)
		}
		page.WriteString(`</div>`)
	}

	return hg.renderPage(outputDir, filepath.Join("pages", "network.html"), "Network Architecture Audit", sidebar, page.String(), 1)
}
