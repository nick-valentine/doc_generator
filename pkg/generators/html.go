package generators

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"doc_generator/pkg/store"
)

// HTMLGenerator is an output plugin that implements store.Generator.
// It formats the parsed files, structs, fields, and methods into a premium self-contained HTML document.
type HTMLGenerator struct{}

// Generate builds a fully styled, premium, single-page HTML application summarizing all files and symbols, and writes it to outputDir.
func (hg *HTMLGenerator) Generate(source *store.Source, outputDir string) error {
	structs := getSymbolsOfKind(source, store.SymStruct)
	interfaces := getSymbolsOfKind(source, store.SymInterface)
	funcs := getSymbolsOfKind(source, store.SymFunction)
	imports := getSymbolsOfKind(source, store.SymImport)

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

	// Group files by package name
	packageMap := make(map[string][]string)
	for _, f := range source.Files {
		pkgName := "main"
		for _, sym := range source.Symbols {
			if sym.File == f.Name && sym.Package != "" {
				pkgName = sym.Package
				break
			}
		}
		if pkgName == "main" {
			dir := filepath.Dir(f.Name)
			if dir != "." && dir != "" {
				pkgName = filepath.Base(dir)
			}
		}
		packageMap[pkgName] = append(packageMap[pkgName], f.Name)
	}

	var sortedPkgs []string
	for pkg := range packageMap {
		sortedPkgs = append(sortedPkgs, pkg)
	}
	sort.Strings(sortedPkgs)

	for _, pkg := range sortedPkgs {
		sort.Strings(packageMap[pkg])
	}

	// Gather unique, alphabetically sorted imports
	var uniqueImports []string
	importMap := make(map[string]bool)
	for _, imp := range imports {
		name := strings.Trim(imp.Name, `"`+`'`+`"`)
		if name == "" {
			continue
		}
		if !importMap[name] {
			importMap[name] = true
			uniqueImports = append(uniqueImports, name)
		}
	}
	sort.Strings(uniqueImports)

	var buf bytes.Buffer

	// HTML Header & Styled CSS
	buf.WriteString(`<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Documentation Dashboard</title>
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
            gap: 2rem;
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
            gap: 0.5rem;
        }

        .nav-section-title {
            font-size: 0.75rem;
            text-transform: uppercase;
            letter-spacing: 0.05em;
            color: var(--text-secondary);
            font-weight: 600;
            margin-bottom: 0.5rem;
        }

        .nav-link {
            color: var(--text-secondary);
            text-decoration: none;
            font-size: 0.9rem;
            padding: 0.5rem 0.75rem;
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
            width: calc(100% - 300px);
        }

        header {
            margin-bottom: 3rem;
            display: flex;
            justify-content: space-between;
            align-items: center;
        }

        h1 {
            font-size: 2.5rem;
            font-weight: 700;
            background: linear-gradient(135deg, #FFFFFF, #9CA3AF);
            -webkit-background-clip: text;
            -webkit-text-fill-color: transparent;
        }

        .stats {
            display: flex;
            gap: 1.5rem;
        }

        .stat-badge {
            background-color: rgba(255, 255, 255, 0.05);
            border: 1px solid var(--border-color);
            padding: 0.5rem 1rem;
            border-radius: 8px;
            font-size: 0.85rem;
            color: var(--text-secondary);
            text-decoration: none;
            display: inline-flex;
            align-items: center;
            cursor: pointer;
            transition: all 0.2s cubic-bezier(0.4, 0, 0.2, 1);
        }

        .stat-badge:hover {
            background-color: rgba(255, 255, 255, 0.1);
            border-color: var(--accent-primary);
            color: var(--text-primary);
            transform: translateY(-2px);
            box-shadow: 0 4px 12px rgba(99, 102, 241, 0.15);
        }

        .stat-badge strong {
            color: var(--text-primary);
        }

        /* Glassmorphic Bento Cards */
        .doc-section {
            margin-bottom: 4rem;
        }

        .doc-section-title {
            font-size: 1.75rem;
            font-weight: 700;
            margin-bottom: 1.5rem;
            border-bottom: 1px solid var(--border-color);
            padding-bottom: 0.5rem;
        }

        .card {
            background-color: var(--card-bg);
            backdrop-filter: var(--glass-blur);
            -webkit-backdrop-filter: var(--glass-blur);
            border: 1px solid var(--border-color);
            border-radius: 12px;
            padding: 2rem;
            margin-bottom: 2rem;
            box-shadow: 0 4px 30px rgba(0, 0, 0, 0.1);
            transition: all 0.3s cubic-bezier(0.4, 0, 0.2, 1);
        }

        .card:hover {
            transform: translateY(-2px);
            border-color: rgba(99, 102, 241, 0.4);
            box-shadow: 0 10px 30px rgba(99, 102, 241, 0.05);
        }

        .card-header {
            display: flex;
            justify-content: space-between;
            align-items: flex-start;
            margin-bottom: 1.5rem;
        }

        .card-title {
            font-size: 1.5rem;
            font-weight: 600;
            color: var(--text-primary);
        }

        .meta-tags {
            display: flex;
            gap: 0.5rem;
            flex-wrap: wrap;
        }

        .tag {
            font-size: 0.75rem;
            font-weight: 600;
            padding: 0.25rem 0.6rem;
            border-radius: 12px;
            text-transform: uppercase;
        }

        .tag-aud {
            background-color: rgba(99, 102, 241, 0.15);
            color: #818CF8;
            border: 1px solid rgba(99, 102, 241, 0.3);
        }

        .tag-comp {
            background-color: rgba(168, 85, 247, 0.15);
            color: #C084FC;
            border: 1px solid rgba(168, 85, 247, 0.3);
        }

        .location {
            font-family: 'Fira Code', monospace;
            font-size: 0.8rem;
            color: var(--text-secondary);
            margin-bottom: 1rem;
        }

        .docblock {
            font-size: 0.95rem;
            line-height: 1.6;
            color: var(--text-secondary);
            border-left: 3px solid var(--accent-primary);
            padding-left: 1rem;
            margin-bottom: 1.5rem;
        }

        /* Sub-sections (Fields/Methods) */
        .sub-grid {
            display: grid;
            grid-template-columns: 1fr;
            gap: 1.5rem;
            margin-top: 1.5rem;
        }

        .sub-section-title {
            font-size: 1.1rem;
            font-weight: 600;
            color: var(--text-primary);
            margin-bottom: 1rem;
            display: flex;
            align-items: center;
            gap: 0.5rem;
        }

        .element-item {
            background-color: rgba(255, 255, 255, 0.02);
            border: 1px solid var(--border-color);
            border-radius: 8px;
            padding: 1.25rem;
            transition: all 0.2s ease;
        }

        .element-item:hover {
            background-color: rgba(255, 255, 255, 0.04);
            border-color: rgba(255, 255, 255, 0.1);
        }

        .element-name {
            font-family: 'Fira Code', monospace;
            font-weight: 500;
            color: var(--text-primary);
            display: flex;
            align-items: center;
            justify-content: space-between;
            margin-bottom: 0.5rem;
        }

        .element-doc {
            font-size: 0.875rem;
            color: var(--text-secondary);
            line-height: 1.5;
            margin-bottom: 0.5rem;
        }

        .call-relations {
            font-size: 0.8rem;
            color: var(--text-secondary);
            margin-top: 0.75rem;
            display: flex;
            flex-direction: column;
            gap: 0.25rem;
        }

        .call-relations strong {
            color: var(--text-primary);
        }

        /* Premium Action Buttons for Graphs */
        .graph-btn {
            display: inline-flex;
            align-items: center;
            justify-content: space-between;
            background: rgba(255, 255, 255, 0.03);
            border: 1px solid var(--border-color);
            color: var(--text-secondary);
            padding: 0.8rem 1.2rem;
            border-radius: 8px;
            text-decoration: none;
            font-size: 0.9rem;
            font-weight: 500;
            cursor: pointer;
            transition: all 0.2s ease-in-out;
            box-shadow: 0 1px 2px rgba(0, 0, 0, 0.05);
        }

        .graph-btn:hover {
            background: rgba(99, 102, 241, 0.08);
            border-color: var(--accent-primary);
            color: var(--text-primary);
            transform: translateY(-2px);
            box-shadow: 0 4px 12px rgba(99, 102, 241, 0.15);
        }

        .graph-btn .arrow {
            margin-left: 0.5rem;
            transition: transform 0.2s;
        }

        .graph-btn:hover .arrow {
            transform: translate(2px, -2px);
        }

        /* Lightbox CSS */
        .lightbox {
            display: none;
            position: fixed;
            z-index: 1000;
            top: 0; left: 0; width: 100%; height: 100%;
            background: rgba(0, 0, 0, 0.85);
            backdrop-filter: blur(8px);
            -webkit-backdrop-filter: blur(8px);
            justify-content: center;
            align-items: center;
            opacity: 0;
            transition: opacity 0.3s ease;
        }
        .lightbox.active {
            display: flex;
            opacity: 1;
        }
        .lightbox img {
            max-width: 90%;
            max-height: 90%;
            border-radius: 8px;
            box-shadow: 0 10px 30px rgba(0,0,0,0.5);
            transform: scale(0.9);
            transition: transform 0.3s cubic-bezier(0.34, 1.56, 0.64, 1);
            background: white;
            padding: 1.5rem;
        }
        .lightbox.active img {
            transform: scale(1);
        }

        /* Compiled Markdown Styling */
        .compiled-markdown h1 {
            font-size: 1.5rem;
            color: var(--text-primary);
            margin: 1.5rem 0 1rem 0;
            border-bottom: 1px solid rgba(255,255,255,0.05);
            padding-bottom: 0.5rem;
        }
        .compiled-markdown h2 {
            font-size: 1.3rem;
            color: var(--text-primary);
            margin: 1.5rem 0 1rem 0;
        }
        .compiled-markdown h3 {
            font-size: 1.15rem;
            color: var(--text-primary);
            margin: 1.2rem 0 0.8rem 0;
        }
        .compiled-markdown p {
            margin-bottom: 1rem;
        }
        .compiled-markdown ul {
            margin-bottom: 1rem;
            padding-left: 1.5rem;
        }
        .compiled-markdown li {
            margin-bottom: 0.4rem;
            list-style-type: disc;
        }
        .compiled-markdown pre {
            background: rgba(0,0,0,0.3);
            border: 1px solid var(--border-color);
            border-radius: 6px;
            padding: 1rem;
            margin: 1rem 0;
            overflow-x: auto;
        }
        .compiled-markdown code {
            font-family: 'Fira Code', monospace;
            font-size: 0.85rem;
            background: rgba(255,255,255,0.04);
            padding: 0.15rem 0.3rem;
            border-radius: 4px;
            color: #818CF8;
        }
        .compiled-markdown pre code {
            background: none;
            padding: 0;
            color: #E2E8F0;
        }

        /* Inline Diagrams */
        .inline-diagram {
            margin-top: 1.5rem;
            background: rgba(0, 0, 0, 0.2);
            border: 1px solid var(--border-color);
            border-radius: 8px;
            padding: 1rem;
            cursor: pointer;
            transition: all 0.2s;
            display: flex;
            justify-content: center;
            align-items: center;
            overflow: hidden;
            max-height: 300px;
        }

        .inline-diagram:hover {
            border-color: var(--accent-primary);
            background: rgba(99, 102, 241, 0.05);
        }

        .inline-diagram img {
            max-width: 100%;
            height: auto;
            filter: brightness(0.9) contrast(1.1);
            transition: transform 0.3s;
        }

        .inline-diagram:hover img {
            transform: scale(1.02);
            filter: brightness(1);
        }

        .diagram-label {
            font-size: 0.75rem;
            color: var(--text-secondary);
            margin-bottom: 0.5rem;
            display: flex;
            align-items: center;
            gap: 0.4rem;
        }
    </style>
</head>
<body>

    <!-- Lightbox Container -->
    <div id="lightbox" class="lightbox" onclick="closeLightbox()">
        <img id="lightbox-img" src="" alt="Call Graph Zoom">
    </div>
`)

	// Sidebar
	buf.WriteString(`    <div class="sidebar">
        <div class="logo">DocGenerator</div>
        
        <div style="padding: 0 1.5rem 1rem 1.5rem;">
            <input type="text" id="searchInput" placeholder="🔍 Search symbols..." style="width: 100%; padding: 0.6rem; border-radius: 6px; border: 1px solid var(--border-color); background: rgba(255,255,255,0.03); color: var(--text-primary); font-size: 0.85rem; outline: none;" oninput="filterDashboard()">
        </div>

        <div class="nav-section">
            <div class="nav-section-title">Packages & Files</div>
`)
	for _, pkg := range sortedPkgs {
		buf.WriteString(fmt.Sprintf(`            <div style="margin-bottom: 0.5rem; padding-left: 0.2rem;">
                <span onclick="filterByPackage('%s')" class="nav-link" style="color: #818CF8; font-weight: 600; display: inline-flex; align-items: center; gap: 0.3rem; cursor: pointer; padding: 0.25rem 0; width: 100%%; box-sizing: border-box; overflow: hidden; text-overflow: ellipsis; white-space: nowrap;">📦 %s</span>
                <div style="padding-left: 0.8rem; border-left: 1px dashed var(--border-color); margin-left: 0.4rem; margin-top: 0.1rem;">`+"\n", pkg, pkg))
		for _, file := range packageMap[pkg] {
			baseName := filepath.Base(file)
			buf.WriteString(fmt.Sprintf(`                    <a href="javascript:void(0)" onclick="selectFile('%s')" class="nav-link" style="font-family: monospace; font-size: 0.8rem; display: block; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; padding: 0.15rem 0;" title="%s">📄 %s</a>`+"\n", file, file, baseName))
		}
		buf.WriteString(`                </div>
            </div>` + "\n")
	}




	buf.WriteString(`        </div>

        <div class="nav-section">
            <div class="nav-section-title">Structures</div>
`)
	for _, s := range structs {
		buf.WriteString(fmt.Sprintf(`            <a href="#struct-%s" class="nav-link">%s</a>`+"\n", s.Name, s.Name))
	}

	buf.WriteString(`        </div>

        <div class="nav-section">
            <div class="nav-section-title">Interfaces</div>
`)
	for _, s := range interfaces {
		buf.WriteString(fmt.Sprintf(`            <a href="#interface-%s" class="nav-link">%s</a>`+"\n", s.Name, s.Name))
	}

	buf.WriteString(`        </div>

        <div class="nav-section">
            <div class="nav-section-title">Global Functions</div>
`)
	for _, fn := range funcs {
		buf.WriteString(fmt.Sprintf(`            <a href="#func-%s" class="nav-link">%s</a>`+"\n", fn.Name, fn.Name))
	}

	buf.WriteString(`        </div>`)

	markdowns := getSymbolsOfKind(source, "markdown")
	if len(markdowns) > 0 {
		buf.WriteString(`
        <div class="nav-section">
            <div class="nav-section-title">Documents</div>
`)
		for _, md := range markdowns {
			cleanID := strings.ReplaceAll(strings.ToLower(md.Name), " ", "-")
			buf.WriteString(fmt.Sprintf(`            <a href="#doc-%s" class="nav-link">📝 %s</a>`+"\n", cleanID, md.Name))
		}
		buf.WriteString(`        </div>
`)
	}

	buf.WriteString(`    </div>`)

	// Main Content Start
	buf.WriteString(`
    <div class="main-content">
        <header>
            <h1>API Reference Dashboard</h1>
            <div style="display: flex; justify-content: space-between; align-items: center; flex-wrap: wrap; gap: 1rem;">
                <div class="stats">
                    <a href="#section-files" class="stat-badge">Files: <strong>` + fmt.Sprintf("%d", len(source.Files)) + `</strong></a>
                    <a href="#section-structures" class="stat-badge">Structs: <strong>` + fmt.Sprintf("%d", len(structs)) + `</strong></a>
                    <a href="#section-functions" class="stat-badge">Functions: <strong>` + fmt.Sprintf("%d", len(funcs)) + `</strong></a>
                    <a href="#section-imports" class="stat-badge">Imports: <strong>` + fmt.Sprintf("%d", len(imports)) + `</strong></a>
                    <a href="#section-todos" class="stat-badge">TODOs: <strong>` + fmt.Sprintf("%d", len(todos)) + `</strong></a>
                </div>

                <div style="display: flex; gap: 1rem; align-items: center; flex-wrap: wrap;">
                    <span style="color: var(--text-secondary); font-size: 0.85rem;">File:</span>
                    <select id="fileFilter" onchange="filterDashboard()" style="background: var(--bg-secondary); border: 1px solid var(--border-color); color: var(--text-primary); padding: 0.4rem; border-radius: 6px; outline: none; font-size: 0.85rem; max-width: 180px;">
                        <option value="all">ALL FILES</option>
	`)
	for _, f := range source.Files {
		buf.WriteString(fmt.Sprintf(`                        <option value="%s">%s</option>`+"\n", f.Name, f.Name))
	}
	buf.WriteString(`
                    </select>

                    <span style="color: var(--text-secondary); font-size: 0.85rem;">Audience:</span>
                    <select id="audFilter" onchange="filterDashboard()" style="background: var(--bg-secondary); border: 1px solid var(--border-color); color: var(--text-primary); padding: 0.4rem; border-radius: 6px; outline: none; font-size: 0.85rem;">
                        <option value="all">ALL</option>
                        <option value="API">API</option>
                        <option value="INTERNAL">INTERNAL</option>
                        <option value="USER">USER</option>
                        <option value="DEVELOPER">DEVELOPER</option>
                    </select>

                    <span style="color: var(--text-secondary); font-size: 0.85rem;">Compatibility:</span>
                    <select id="compFilter" onchange="filterDashboard()" style="background: var(--bg-secondary); border: 1px solid var(--border-color); color: var(--text-primary); padding: 0.4rem; border-radius: 6px; outline: none; font-size: 0.85rem;">
                        <option value="all">ALL</option>
                        <option value="C">C</option>
                        <option value="RUST">RUST</option>
                        <option value="JS">JS</option>
                    </select>
                </div>
            </div>
        </header>
	`)

	// Dashboard Overview Grid
	buf.WriteString(`
        <div style="display: grid; grid-template-columns: 1fr 1fr; gap: 2rem; margin-bottom: 3rem;">
            <!-- Module Tree & Imports Card -->
            <div class="card" id="section-files" style="margin-bottom: 0; display: flex; flex-direction: column; justify-content: space-between;">
                <div>
                    <div class="card-header" style="margin-bottom: 1rem;">
                        <div class="card-title" style="font-size: 1.2rem;">Module Tree & Import Graphs</div>
                    </div>
                    <div style="font-size: 0.85rem; color: var(--text-secondary); max-height: 250px; overflow-y: auto; padding-right: 0.5rem; margin-bottom: 1rem;">
                        <strong style="color: var(--text-primary); display: block; margin-bottom: 0.75rem;">📁 Module Tree (by Package)</strong>
                        <ul style="list-style: none; padding-left: 0; line-height: 1.6; font-family: monospace;">
	`)

	for _, pkg := range sortedPkgs {
		buf.WriteString(fmt.Sprintf(`                            <li style="margin-bottom: 0.75rem;">
                                <strong style="color: #818CF8; font-size: 0.9rem; display: flex; align-items: center; gap: 0.3rem; cursor: pointer;" onclick="filterByPackage('%s')">📦 %s</strong>
                                <ul style="list-style: none; padding-left: 1.2rem; margin-top: 0.2rem; border-left: 1px dashed var(--border-color); margin-left: 0.4rem;">`+"\n", pkg, pkg))
		for _, file := range packageMap[pkg] {
			baseName := filepath.Base(file)
			buf.WriteString(fmt.Sprintf(`                                    <li style="margin: 0.15rem 0;"><a href="javascript:void(0)" onclick="selectFile('%s')" style="color: var(--text-secondary); text-decoration: none; display: inline-flex; align-items: center; gap: 0.3rem;" onmouseover="this.style.color='var(--text-primary)'" onmouseout="this.style.color='var(--text-secondary)'">📄 %s</a></li>`+"\n", file, baseName))
		}
		buf.WriteString(`                                </ul>
                            </li>` + "\n")
	}

	buf.WriteString(`                        </ul>
                    </div>
                </div>

                <div>
                    <!-- Unique Imported Packages (horizontal inline, less prominent place) -->
                    <div id="section-imports" style="border-top: 1px solid var(--border-color); padding-top: 0.75rem; margin-top: 0.5rem;">
                        <strong style="color: var(--text-primary); font-size: 0.8rem; display: block; margin-bottom: 0.4rem;">🔗 Unique Imported Packages</strong>
                        <div style="display: flex; flex-wrap: wrap; gap: 0.4rem; max-height: 120px; overflow-y: auto;">
	`)

	if len(uniqueImports) == 0 {
		buf.WriteString(`                            <span style="font-size: 0.75rem; color: var(--text-secondary); font-style: italic;">(None)</span>` + "\n")
	} else {
		for _, imp := range uniqueImports {
			buf.WriteString(fmt.Sprintf(`                            <span style="background: var(--bg-secondary); border: 1px solid var(--border-color); padding: 0.15rem 0.4rem; border-radius: 4px; font-size: 0.75rem; color: #818CF8; font-family: monospace;">%s</span>`+"\n", imp))
		}
	}

	buf.WriteString(`                        </div>
                    </div>

                    <div style="margin-top: 1.2rem; border-top: 1px solid var(--border-color); padding-top: 1rem; display: grid; grid-template-columns: 1fr 1fr 1fr; gap: 1rem;">
                        <a href="graphs/imports.html" class="graph-btn" target="_blank" style="padding: 0.5rem; font-size: 0.75rem;">
                            <span>📊 Import Graph</span>
                            <span class="arrow">↗</span>
                        </a>
                        <a href="graphs/program.html" class="graph-btn" target="_blank" style="padding: 0.5rem; font-size: 0.75rem;">
                            <span>🟢 Callee Graph</span>
                            <span class="arrow">↗</span>
                        </a>
                        <a href="graphs/relations.html" class="graph-btn" target="_blank" style="padding: 0.5rem; font-size: 0.75rem;">
                            <span>🧬 Type Graph</span>
                            <span class="arrow">↗</span>
                        </a>
                    </div>
                </div>
            </div>

            <!-- Code Metrics & CRAP Index Card -->
            <div class="card" style="margin-bottom: 0;">
                <div class="card-header" style="margin-bottom: 1rem;">
                    <div class="card-title" style="font-size: 1.2rem;">CRAP Index & Large Functions</div>
                </div>
                <div style="max-height: 250px; overflow-y: auto;">
                    <table class="metric-table" style="width: 100%; border-collapse: collapse; text-align: left; font-size: 0.8rem; color: var(--text-secondary);">
                        <thead>
                            <tr style="border-bottom: 2px solid var(--border-color); color: var(--text-primary);">
                                <th style="padding: 0.4rem;">Name</th>
                                <th style="padding: 0.4rem;">Lines</th>
                                <th style="padding: 0.4rem;">Complexity</th>
                                <th style="padding: 0.4rem;">CRAP</th>
                                <th style="padding: 0.4rem;">Status</th>
                            </tr>
                        </thead>
                        <tbody>
	`)

	// Gather all functions and methods with their computed CRAP scores for sorting
	type CrapEntry struct {
		Symbol      store.Symbol
		DisplayName string
		CrapScore   int
	}
	var crapList []CrapEntry
	for _, fn := range funcs {
		crap := fn.Complexity*fn.Complexity + fn.Complexity
		crapList = append(crapList, CrapEntry{
			Symbol:      fn,
			DisplayName: fn.Name + "()",
			CrapScore:   crap,
		})
	}
	for _, s := range structs {
		methods := source.GetStructMethods(s.Name)
		for _, m := range methods {
			crap := m.Complexity*m.Complexity + m.Complexity
			crapList = append(crapList, CrapEntry{
				Symbol:      m,
				DisplayName: s.Name + "." + m.Name + "()",
				CrapScore:   crap,
			})
		}
	}

	// Sort from most crappy to least crappy
	sort.Slice(crapList, func(i, j int) bool {
		return crapList[i].CrapScore > crapList[j].CrapScore
	})

	if len(crapList) == 0 {
		buf.WriteString(`                            <tr><td colspan="5" style="text-align: center; padding: 1rem;">No functions or methods found.</td></tr>` + "\n")
	} else {
		for _, entry := range crapList {
			status := `<span style="color: #10B981; font-weight: 600;">Good</span>`
			if entry.CrapScore > 20 || entry.Symbol.LineCount > 50 {
				status = `<span style="color: #F59E0B; font-weight: 600;">Complex</span>`
			}
			if entry.CrapScore > 50 {
				status = `<span style="color: #EF4444; font-weight: 600;">CRITICAL</span>`
			}

			anchor := "func-" + entry.Symbol.Name
			if entry.Symbol.Parent != "" {
				anchor = fmt.Sprintf("struct-%s-method-%s", entry.Symbol.Parent, entry.Symbol.Name)
			}

			buf.WriteString(fmt.Sprintf(`                            <tr class="metric-row" data-file="%s">
                                <td style="padding: 0.4rem; font-family: monospace; color: var(--text-primary);"><a href="#%s" style="color: inherit; text-decoration: none;">%s</a></td>
                                <td style="padding: 0.4rem;">%d</td>
                                <td style="padding: 0.4rem;">%d</td>
                                <td style="padding: 0.4rem;">%d</td>
                                <td style="padding: 0.4rem;">%s</td>
                            </tr>`+"\n", entry.Symbol.File, anchor, entry.DisplayName, entry.Symbol.LineCount, entry.Symbol.Complexity, entry.CrapScore, status))
		}
	}

	buf.WriteString(`                        </tbody>
                    </table>
                </div>
            </div>
        </div>
	`)

	// TODO Index Card
	if len(todos) > 0 {
		buf.WriteString(`
        <div class="card" id="section-todos" style="margin-bottom: 3rem;">
            <div class="card-header" style="margin-bottom: 1rem;">
                <div class="card-title" style="font-size: 1.2rem; color: #F59E0B;">⚠️ TODO Index</div>
            </div>
            <div style="max-height: 200px; overflow-y: auto; font-family: monospace; font-size: 0.85rem;">
                <table style="width: 100%; border-collapse: collapse; text-align: left; color: var(--text-secondary);">
                    <thead>
                        <tr style="border-bottom: 1px solid var(--border-color); color: var(--text-primary);">
                            <th style="padding: 0.4rem;">File</th>
                            <th style="padding: 0.4rem;">Line</th>
                            <th style="padding: 0.4rem;">Description</th>
                        </tr>
                    </thead>
                    <tbody>
		`)
		for _, todo := range todos {
			buf.WriteString(fmt.Sprintf(`                        <tr class="todo-row" data-file="%s" style="border-bottom: 1px solid rgba(255,255,255,0.02);">
                            <td style="padding: 0.4rem; color: var(--text-primary);">%s</td>
                            <td style="padding: 0.4rem;">%d</td>
                            <td style="padding: 0.4rem; color: #F8FAFC;">%s</td>
                        </tr>`+"\n", todo.File, todo.File, todo.Line, todo.Doc))
		}
		buf.WriteString(`
                    </tbody>
                </table>
            </div>
        </div>
		`)
	}

	// Global Variables & Constants Card
	if len(globals) > 0 {
		buf.WriteString(`
        <div class="card" style="margin-bottom: 3rem;">
            <div class="card-header" style="margin-bottom: 1rem;">
                <div class="card-title" style="font-size: 1.2rem;">📦 Global Variables & Constants Index</div>
            </div>
            <div style="max-height: 200px; overflow-y: auto; font-family: monospace; font-size: 0.85rem;">
                <table style="width: 100%; border-collapse: collapse; text-align: left; color: var(--text-secondary);">
                    <thead>
                        <tr style="border-bottom: 1px solid var(--border-color); color: var(--text-primary);">
                            <th style="padding: 0.4rem;">Name</th>
                            <th style="padding: 0.4rem;">Type</th>
                            <th style="padding: 0.4rem;">Location</th>
                        </tr>
                    </thead>
                    <tbody>
		`)
		for _, v := range globals {
			typed := renderTypeWithLinks(v.Type, source)
			buf.WriteString(fmt.Sprintf(`                        <tr class="variable-row" data-file="%s" style="border-bottom: 1px solid rgba(255,255,255,0.02);">
                            <td style="padding: 0.4rem; color: var(--text-primary);">%s</td>
                            <td style="padding: 0.4rem; color: #818CF8;">%s</td>
                            <td style="padding: 0.4rem;">%s:%d</td>
                        </tr>`+"\n", v.File, v.Name, typed, v.File, v.Line))
		}
		buf.WriteString(`
                    </tbody>
                </table>
            </div>
        </div>
		`)
	}

	// Documents Section
	if len(markdowns) > 0 {
		buf.WriteString(`        <div class="doc-section" id="section-documents">
            <div class="doc-section-title">Documents</div>
`)
		for _, md := range markdowns {
			cleanID := strings.ReplaceAll(strings.ToLower(md.Name), " ", "-")
			buf.WriteString(fmt.Sprintf(`            <div class="card" id="doc-%s" data-file="%s">
                <div class="card-header" style="border-bottom: 1px solid var(--border-color); padding-bottom: 0.75rem; margin-bottom: 1.5rem;">
                    <div class="card-title" style="font-size: 1.25rem;">📝 %s</div>
                    <div class="location" style="font-family: monospace; font-size: 0.8rem; color: var(--text-muted);">%s</div>
                </div>
                <div class="compiled-markdown" style="color: var(--text-secondary); line-height: 1.7; font-size: 0.95rem;">
                    %s
                </div>
            </div>`+"\n", cleanID, md.File, md.Name, md.File, renderMarkdownToHTML(md.Doc)))
		}
		buf.WriteString(`        </div>` + "\n")
	}

	// Structures Section
	buf.WriteString(`        <div class="doc-section" id="section-structures">
            <div class="doc-section-title">Structures</div>
`)
	for _, s := range structs {
		buf.WriteString(fmt.Sprintf(`            <div class="card" id="struct-%s" data-file="%s">
                <div class="card-header">
                    <div class="card-title">%s</div>
                    <div class="meta-tags">`+"\n", s.Name, s.File, s.Name))

		for _, aud := range s.Audience {
			buf.WriteString(fmt.Sprintf(`                        <span class="tag tag-aud">%s</span>`+"\n", aud))
		}
		for _, comp := range s.Compatibility {
			buf.WriteString(fmt.Sprintf(`                        <span class="tag tag-comp">%s</span>`+"\n", comp))
		}

		buf.WriteString(`                    </div>
                </div>`)

		fields := source.GetStructFields(s.Name)
		methods := source.GetStructMethods(s.Name)

		// Compute relationships dynamically
		var relations []string

		// Check for implements
		for _, m := range methods {
			if m.Name == "Parse" {
				relations = append(relations, `👉 implements <a href="#interface-Parser" style="color: #34D399; font-weight: 600;">Parser</a>`)
			}
			if m.Name == "Generate" {
				relations = append(relations, `👉 implements <a href="#interface-Generator" style="color: #34D399; font-weight: 600;">Generator</a>`)
			}
		}

		// Check for embedding / composition
		for _, f := range fields {
			if f.Name == f.Type || strings.HasSuffix(f.Type, f.Name) {
				relations = append(relations, fmt.Sprintf(`🧬 embeds (composition) <a href="#struct-%s" style="color: #818CF8; font-weight: 600;">%s</a>`, f.Name, f.Name))
			} else {
				for _, other := range structs {
					if other.Name != s.Name && (f.Type == other.Name || f.Type == "*"+other.Name || strings.HasSuffix(f.Type, other.Name)) {
						relations = append(relations, fmt.Sprintf(`🧩 composed of <a href="#struct-%s" style="color: #6366F1; font-weight: 600;">%s</a> (via field %s)`, other.Name, other.Name, f.Name))
					}
				}
			}
		}

		// Explicit relations from the parser
		for _, rel := range s.Relations {
			// Find if it's a struct or interface
			found := false
			for _, other := range structs {
				if other.Name == rel {
					relations = append(relations, fmt.Sprintf(`🔗 relates to <a href="#struct-%s" style="color: #818CF8; font-weight: 600;">%s</a>`, other.Name, other.Name))
					found = true
					break
				}
			}
			if !found {
				for _, other := range interfaces {
					if other.Name == rel {
						relations = append(relations, fmt.Sprintf(`🔌 implements/uses <a href="#interface-%s" style="color: #34D399; font-weight: 600;">%s</a>`, other.Name, other.Name))
						break
					}
				}
			}
		}

		buf.WriteString(fmt.Sprintf(`                <div class="location">%s (Line %d)</div>`+"\n", s.File, s.Line))

		hasStateChange := false
		for _, m := range methods {
			name := strings.ToLower(m.Name)
			if strings.Contains(name, "init") || strings.Contains(name, "parse") || strings.Contains(name, "generate") || strings.Contains(name, "run") || strings.Contains(name, "close") || strings.Contains(name, "stop") {
				hasStateChange = true
				break
			}
		}

		buf.WriteString(`                <div style="margin: 0.5rem 0 1rem 0; font-size: 0.85rem; color: var(--text-secondary); display: flex; flex-wrap: wrap; gap: 1rem; align-items: center;">` + "\n")
		for _, rel := range relations {
			buf.WriteString(fmt.Sprintf(`                    <div style="background: var(--bg-secondary); border: 1px solid var(--border-color); padding: 0.25rem 0.6rem; border-radius: 6px; display: inline-flex; align-items: center; gap: 0.35rem;">%s</div>`+"\n", rel))
		}
		if hasStateChange {
			buf.WriteString(fmt.Sprintf(`                    <a href="graphs/%s_timing.html" class="graph-btn" target="_blank" style="font-size: 0.8rem; padding: 0.25rem 0.6rem; margin-left: auto; background: rgba(245, 158, 11, 0.1); border-color: rgba(245, 158, 11, 0.2); color: #F59E0B;">
                        <span>⏳ View Lifecycle Timing</span>
                        <span class="arrow">↗</span>
                    </a>
                    <a href="graphs/%s_type.html" class="graph-btn" target="_blank" style="font-size: 0.8rem; padding: 0.25rem 0.6rem;">
                        <span>🧬 View Relationship Graph</span>
                        <span class="arrow">↗</span>
                    </a>`+"\n", s.Name, s.Name))
		} else {
			buf.WriteString(fmt.Sprintf(`                    <a href="graphs/%s_type.html" class="graph-btn" target="_blank" style="font-size: 0.8rem; padding: 0.25rem 0.6rem; margin-left: auto;">
                        <span>🧬 View Relationship Graph</span>
                        <span class="arrow">↗</span>
                    </a>`+"\n", s.Name))
		}
		buf.WriteString(`                </div>` + "\n")

		// Inline Relationship Diagram
		buf.WriteString(fmt.Sprintf(`                <div class="diagram-label" style="margin-top: 1.5rem;"><span>🧬 Relationship Diagram</span> <span style="font-size: 0.7rem; opacity: 0.5;">(Click to zoom)</span></div>
                <div class="inline-diagram" onclick="openLightbox('images/%s_type_graph.png')">
                    <img src="images/%s_type_graph.png" alt="%s Relationship Graph" onerror="this.parentElement.style.display='none'; this.parentElement.previousElementSibling.style.display='none';">
                </div>`+"\n", s.Name, s.Name, s.Name))

		if hasStateChange {
			buf.WriteString(fmt.Sprintf(`                <div class="diagram-label" style="margin-top: 1.5rem;"><span>⏳ Struct Lifecycle Timing Diagram</span> <span style="font-size: 0.7rem; opacity: 0.5;">(Click to zoom)</span></div>
                <div class="inline-diagram" onclick="openLightbox('images/%s_timing.png')">
                    <img src="images/%s_timing.png" alt="%s Lifecycle Timing" onerror="this.parentElement.style.display='none'; this.parentElement.previousElementSibling.style.display='none';">
                </div>`+"\n", s.Name, s.Name, s.Name))
		}

		if s.Doc != "" {
			cleanDoc := strings.ReplaceAll(strings.TrimSpace(s.Doc), "\n", "<br>")
			buf.WriteString(fmt.Sprintf(`                <div class="docblock">%s</div>`+"\n", cleanDoc))
		}
		if len(fields) > 0 {
			buf.WriteString(`                <div class="sub-grid">
                    <div class="sub-section-title">Fields</div>` + "\n")
			for _, f := range fields {
				typed := renderTypeWithLinks(f.Type, source)
				buf.WriteString(fmt.Sprintf(`                    <div class="element-item">
                        <div class="element-name">
                            <span>%s <span style="font-weight: 400; color: var(--text-secondary); font-size: 0.9rem; font-family: 'Fira Code', monospace; margin-left: 0.5rem;">%s</span></span>
                            <div class="meta-tags">`+"\n", f.Name, typed))
				for _, aud := range f.Audience {
					buf.WriteString(fmt.Sprintf(`                                <span class="tag tag-aud">%s</span>`+"\n", aud))
				}
				for _, comp := range f.Compatibility {
					buf.WriteString(fmt.Sprintf(`                                <span class="tag tag-comp">%s</span>`+"\n", comp))
				}
				buf.WriteString(`                            </div>
                        </div>` + "\n")
				if f.Doc != "" {
					cleanDoc := strings.ReplaceAll(strings.TrimSpace(f.Doc), "\n", "<br>")
					buf.WriteString(fmt.Sprintf(`                        <div class="element-doc">%s</div>`+"\n", cleanDoc))
				}
				buf.WriteString(`                    </div>` + "\n")
			}
			buf.WriteString(`                </div>` + "\n")
		}
		if len(methods) > 0 {
			buf.WriteString(`                <div class="sub-grid" style="margin-top: 2rem;">
                    <div class="sub-section-title">Methods</div>` + "\n")
			for _, m := range methods {
				params := renderTypeWithLinks(m.Params, source)
				returns := renderTypeWithLinks(m.Returns, source)
				sig := fmt.Sprintf(`<span style="font-family: 'Fira Code', monospace; font-size: 0.95rem; font-weight: 400; color: var(--text-secondary); margin-left: 0.5rem;">%s %s</span>`, params, returns)
				crap := m.Complexity*m.Complexity + m.Complexity
				var crapBadge string
				if crap > 50 {
					crapBadge = fmt.Sprintf(`<span class="tag" style="background: rgba(239, 68, 68, 0.15); color: #EF4444; border: 1px solid rgba(239, 68, 68, 0.3); font-weight: 600;">CRAP: %d (Critical)</span>`, crap)
				} else if crap > 20 {
					crapBadge = fmt.Sprintf(`<span class="tag" style="background: rgba(245, 158, 11, 0.15); color: #F59E0B; border: 1px solid rgba(245, 158, 11, 0.3); font-weight: 600;">CRAP: %d (Complex)</span>`, crap)
				} else {
					crapBadge = fmt.Sprintf(`<span class="tag" style="background: rgba(16, 185, 129, 0.15); color: #10B981; border: 1px solid rgba(16, 185, 129, 0.3); font-weight: 600;">CRAP: %d</span>`, crap)
				}

				buf.WriteString(fmt.Sprintf(`                    <div class="element-item" id="struct-%s-method-%s">
                        <div class="element-name">
                            <span>%s%s</span>
                            <div class="meta-tags">
                                %s`+"\n", s.Name, m.Name, m.Name, sig, crapBadge))
				for _, aud := range m.Audience {
					buf.WriteString(fmt.Sprintf(`                                <span class="tag tag-aud">%s</span>`+"\n", aud))
				}
				for _, comp := range m.Compatibility {
					buf.WriteString(fmt.Sprintf(`                                <span class="tag tag-comp">%s</span>`+"\n", comp))
				}
				buf.WriteString(`                            </div>
                        </div>` + "\n")
				if m.Doc != "" {
					cleanDoc := strings.ReplaceAll(strings.TrimSpace(m.Doc), "\n", "<br>")
					buf.WriteString(fmt.Sprintf(`                        <div class="element-doc">%s</div>`+"\n", cleanDoc))
				}

				methodKey := fmt.Sprintf("%s.%s", s.Name, m.Name)
				qualifiedKey := methodKey
				if s.Package != "" {
					qualifiedKey = s.Package + "." + methodKey
				}
				callers := source.GetCallers(qualifiedKey)
				callees := source.GetCallees(qualifiedKey)
				cleanKey := strings.ReplaceAll(qualifiedKey, ".", "_")

				if len(callers) > 0 || len(callees) > 0 {
					buf.WriteString(`                        <div class="call-relations">` + "\n")
					if len(callers) > 0 {
						buf.WriteString(fmt.Sprintf(`                            <div><strong>Callers:</strong> %s</div>`+"\n", strings.Join(callers, ", ")))
					}
					if len(callees) > 0 {
						buf.WriteString(fmt.Sprintf(`                            <div><strong>Callees:</strong> %s</div>`+"\n", strings.Join(callees, ", ")))
					}
					buf.WriteString(`                        </div>` + "\n")

					// Inline Call Diagram
					buf.WriteString(fmt.Sprintf(`                        <div class="inline-diagram" style="max-height: 200px; margin-top: 1rem;" onclick="openLightbox('images/%s_call_graph.png')">
                            <img src="images/%s_call_graph.png" alt="%s Call Graph" onerror="this.parentElement.style.display='none';">
                        </div>`+"\n", cleanKey, cleanKey, methodKey))

					buf.WriteString(fmt.Sprintf(`                        <div style="margin-top: 1rem; display: flex; justify-content: flex-end;">
                            <a href="graphs/%s_call.html" class="graph-btn" target="_blank" style="font-size: 0.8rem; padding: 0.25rem 0.6rem;">
                                <span>🟢 View Call Graph</span>
                                <span class="arrow">↗</span>
                            </a>
                        </div>`+"\n", cleanKey))
				}

				buf.WriteString(`                    </div>` + "\n")
			}
			buf.WriteString(`                </div>` + "\n")
		}

		buf.WriteString(`            </div>` + "\n")
	}
	buf.WriteString(`        </div>` + "\n")

	// Interfaces Section
	if len(interfaces) > 0 {
		buf.WriteString(`        <div class="doc-section">
            <div class="doc-section-title">Interfaces</div>` + "\n")
	for _, s := range interfaces {
		buf.WriteString(fmt.Sprintf(`            <div class="card" id="interface-%s" data-file="%s">
                <div class="card-header">
                    <div class="card-title">%s</div>
                    <div class="meta-tags">`+"\n", s.Name, s.File, s.Name))

		for _, aud := range s.Audience {
			buf.WriteString(fmt.Sprintf(`                        <span class="tag tag-aud">%s</span>`+"\n", aud))
		}
		for _, comp := range s.Compatibility {
			buf.WriteString(fmt.Sprintf(`                        <span class="tag tag-comp">%s</span>`+"\n", comp))
		}

		// Show implementors
		var implementors []string
		for _, other := range structs {
			for _, rel := range other.Relations {
				if rel == s.Name {
					implementors = append(implementors, fmt.Sprintf(`<a href="#struct-%s" style="color: #818CF8; font-weight: 600;">%s</a>`, other.Name, other.Name))
				}
			}
		}

		buf.WriteString(fmt.Sprintf(`                    </div>
                </div>
                <div class="location">%s (Line %d)</div>`+"\n", s.File, s.Line))

		if len(implementors) > 0 {
			buf.WriteString(`                <div style="margin: 0.5rem 0 1rem 0; font-size: 0.85rem; color: var(--text-secondary);">
                    📥 <strong>Implemented by:</strong> ` + strings.Join(implementors, ", ") + `
                </div>` + "\n")
		}

		buf.WriteString(fmt.Sprintf(`                <div style="margin: 0.5rem 0 1rem 0; font-size: 0.85rem; color: var(--text-secondary); display: flex; flex-wrap: wrap; gap: 1rem; align-items: center;">
                    <a href="graphs/%s_type.html" class="graph-btn" target="_blank" style="font-size: 0.8rem; padding: 0.25rem 0.6rem; margin-left: auto;">
                        <span>🧬 View Relationship Graph</span>
                        <span class="arrow">↗</span>
                    </a>
                </div>`+"\n", s.Name))

		// Inline Interface Diagram
		buf.WriteString(fmt.Sprintf(`                <div class="diagram-label" style="margin-top: 1.5rem;"><span>📥 Implementation Diagram</span> <span style="font-size: 0.7rem; opacity: 0.5;">(Click to zoom)</span></div>
                <div class="inline-diagram" onclick="openLightbox('images/%s_type_graph.png')">
                    <img src="images/%s_type_graph.png" alt="%s Implementation Graph" onerror="this.parentElement.style.display='none'; this.parentElement.previousElementSibling.style.display='none';">
                </div>`+"\n", s.Name, s.Name, s.Name))

		if s.Doc != "" {
			cleanDoc := strings.ReplaceAll(strings.TrimSpace(s.Doc), "\n", "<br>")
			buf.WriteString(fmt.Sprintf(`                <div class="docblock">%s</div>`+"\n", cleanDoc))
		}
		buf.WriteString(`            </div>` + "\n")
	}
	buf.WriteString(`        </div>` + "\n")
	}

	// Global Functions Section
	buf.WriteString(`        <div class="doc-section" id="section-functions">
            <div class="doc-section-title">Global Functions</div>
`)
	for _, fn := range funcs {
		params := renderTypeWithLinks(fn.Params, source)
		returns := renderTypeWithLinks(fn.Returns, source)
		sig := fmt.Sprintf(`<span style="font-family: 'Fira Code', monospace; font-size: 1rem; font-weight: 400; color: var(--text-secondary); margin-left: 0.5rem;">%s %s</span>`, params, returns)
		crap := fn.Complexity*fn.Complexity + fn.Complexity
		var crapBadge string
		if crap > 50 {
			crapBadge = fmt.Sprintf(`<span class="tag" style="background: rgba(239, 68, 68, 0.15); color: #EF4444; border: 1px solid rgba(239, 68, 68, 0.3); font-weight: 600;">CRAP: %d (Critical)</span>`, crap)
		} else if crap > 20 {
			crapBadge = fmt.Sprintf(`<span class="tag" style="background: rgba(245, 158, 11, 0.15); color: #F59E0B; border: 1px solid rgba(245, 158, 11, 0.3); font-weight: 600;">CRAP: %d (Complex)</span>`, crap)
		} else {
			crapBadge = fmt.Sprintf(`<span class="tag" style="background: rgba(16, 185, 129, 0.15); color: #10B981; border: 1px solid rgba(16, 185, 129, 0.3); font-weight: 600;">CRAP: %d</span>`, crap)
		}

		buf.WriteString(fmt.Sprintf(`            <div class="card" id="func-%s" data-file="%s">
                <div class="card-header">
                    <div class="card-title">%s%s</div>
                    <div class="meta-tags">
                        %s`+"\n", fn.Name, fn.File, fn.Name, sig, crapBadge))
		for _, aud := range fn.Audience {
			buf.WriteString(fmt.Sprintf(`                        <span class="tag tag-aud">%s</span>`+"\n", aud))
		}
		for _, comp := range fn.Compatibility {
			buf.WriteString(fmt.Sprintf(`                        <span class="tag tag-comp">%s</span>`+"\n", comp))
		}
		buf.WriteString(fmt.Sprintf(`                    </div>
                </div>
                <div class="location">%s (Line %d)</div>`+"\n", fn.File, fn.Line))

		if fn.Doc != "" {
			cleanDoc := strings.ReplaceAll(strings.TrimSpace(fn.Doc), "\n", "<br>")
			buf.WriteString(fmt.Sprintf(`                <div class="docblock">%s</div>`+"\n", cleanDoc))
		}

		fnKey := fn.Name
		if fn.Parent != "" {
			fnKey = fn.Parent + "." + fn.Name
		}
		if fn.Package != "" {
			fnKey = fn.Package + "." + fnKey
		}
		callers := source.GetCallers(fnKey)
		callees := source.GetCallees(fnKey)
		cleanKey := strings.ReplaceAll(fnKey, ".", "_")

		if len(callers) > 0 || len(callees) > 0 {
			buf.WriteString(`                <div class="call-relations">` + "\n")
			if len(callers) > 0 {
				buf.WriteString(fmt.Sprintf(`                    <div><strong>Callers:</strong> %s</div>`+"\n", strings.Join(callers, ", ")))
			}
			if len(callees) > 0 {
				buf.WriteString(fmt.Sprintf(`                    <div><strong>Callees:</strong> %s</div>`+"\n", strings.Join(callees, ", ")))
			}
			buf.WriteString(`                </div>` + "\n")

			// Inline Call Diagram & Sequence Diagram
			buf.WriteString(fmt.Sprintf(`                <div class="diagram-label" style="margin-top: 1.5rem;"><span>🟢 Call Graph</span></div>
                <div class="inline-diagram" style="max-height: 250px;" onclick="openLightbox('images/%s_call_graph.png')">
                    <img src="images/%s_call_graph.png" alt="%s Call Graph" onerror="this.parentElement.style.display='none'; this.parentElement.previousElementSibling.style.display='none';">
                </div>
                <div class="diagram-label" style="margin-top: 1.5rem;"><span>📋 Sequence Diagram</span></div>
                <div class="inline-diagram" style="max-height: 250px;" onclick="openLightbox('images/%s_sequence.png')">
                    <img src="images/%s_sequence.png" alt="%s Sequence Diagram" onerror="this.parentElement.style.display='none'; this.parentElement.previousElementSibling.style.display='none';">
                </div>`+"\n", cleanKey, cleanKey, fn.Name, cleanKey, cleanKey, fn.Name))

			buf.WriteString(fmt.Sprintf(`                <div style="margin-top: 1rem; display: flex; justify-content: flex-end; gap: 0.5rem;">
                    <a href="graphs/%s_sequence.html" class="graph-btn" target="_blank" style="font-size: 0.8rem; padding: 0.25rem 0.6rem; background: rgba(16, 185, 129, 0.1); border-color: rgba(16, 185, 129, 0.2); color: #10B981;">
                        <span>📋 View Sequence Diagram</span>
                        <span class="arrow">↗</span>
                    </a>
                    <a href="graphs/%s_call.html" class="graph-btn" target="_blank" style="font-size: 0.8rem; padding: 0.25rem 0.6rem;">
                        <span>🟢 View Call Graph</span>
                        <span class="arrow">↗</span>
                    </a>
                </div>`+"\n", cleanKey, cleanKey))
		}

		buf.WriteString(`            </div>` + "\n")
	}

	buf.WriteString(`        </div>
    </div>`)

	// Interactive Vanilla Javascript Modal & Filters
	buf.WriteString(`
    <script>
        function openLightbox(src) {
            const box = document.getElementById('lightbox');
            const img = document.getElementById('lightbox-img');
            img.src = src;
            box.classList.add('active');
        }

        function closeLightbox() {
            document.getElementById('lightbox').classList.remove('active');
        }

        function selectFile(fileName) {
            const fileSelector = document.getElementById('fileFilter');
            if (fileSelector) {
                fileSelector.value = fileName;
                filterDashboard();
            }
        }

        function filterByPackage(pkgName) {
            const searchInput = document.getElementById('searchInput');
            if (searchInput) {
                searchInput.value = pkgName + ".";
                filterDashboard();
            }
        }

        function filterDashboard() {
            const query = document.getElementById('searchInput').value.toLowerCase();
            const audFilter = document.getElementById('audFilter').value.toLowerCase();
            const compFilter = document.getElementById('compFilter').value.toLowerCase();
            const fileFilter = document.getElementById('fileFilter').value;

            const cards = document.querySelectorAll('.card');
            cards.forEach(card => {
                if (!card.id) return;

                const titleNode = card.querySelector('.card-title');
                const title = titleNode ? titleNode.textContent.toLowerCase() : '';
                const docNode = card.querySelector('.docblock');
                const doc = docNode ? docNode.textContent.toLowerCase() : '';

                const audTags = card.querySelectorAll('.tag-aud');
                let matchesAud = (audFilter === 'all');
                if (!matchesAud) {
                    audTags.forEach(tag => {
                        if (tag.textContent.toLowerCase() === audFilter) matchesAud = true;
                    });
                }

                const compTags = card.querySelectorAll('.tag-comp');
                let matchesComp = (compFilter === 'all');
                if (!matchesComp) {
                    compTags.forEach(tag => {
                        if (tag.textContent.toLowerCase() === compFilter) matchesComp = true;
                    });
                }

                const cardFile = card.getAttribute('data-file') || '';
                const matchesFile = (fileFilter === 'all' || cardFile === fileFilter);

                const matchesQuery = title.includes(query) || doc.includes(query);

                if (matchesAud && matchesComp && matchesFile && matchesQuery) {
                    card.style.display = 'block';
                } else {
                    card.style.display = 'none';
                }
            });

            const navLinks = document.querySelectorAll('.sidebar .nav-link');
            navLinks.forEach(link => {
                const text = link.textContent.toLowerCase();
                const matchesQuery = text.includes(query);
                if (matchesQuery) {
                    link.style.display = 'block';
                } else {
                    link.style.display = 'none';
                }
            });

            const rows = document.querySelectorAll('.metric-row, .todo-row, .variable-row');
            rows.forEach(row => {
                const rowFile = row.getAttribute('data-file') || '';
                const matchesFile = (fileFilter === 'all' || rowFile === fileFilter);
                if (matchesFile) {
                    row.style.display = '';
                } else {
                    row.style.display = 'none';
                }
            });
        }
    </script>
</body>
</html>
`)
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(outputDir, "index.html"), buf.Bytes(), 0644)
}

// renderTypeWithLinks wraps any matching struct type name with a clickable anchor link.
func renderTypeWithLinks(typeStr string, source *store.Source) string {
	if typeStr == "" {
		return ""
	}
	escaped := typeStr
	escaped = strings.ReplaceAll(escaped, "<", "&lt;")
	escaped = strings.ReplaceAll(escaped, ">", "&gt;")

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
				linked := fmt.Sprintf(`<a href="#struct-%s" style="color: #818CF8; text-decoration: underline; text-underline-offset: 4px; font-weight: 500;">%s</a>`, s.Name, s.Name)
				words[i] = strings.Replace(word, s.Name, linked, 1)
				matched = true
				break
			}
		}
		if !matched {
			for _, s := range interfaces {
				if cleanWord == s.Name {
					linked := fmt.Sprintf(`<a href="#interface-%s" style="color: #34D399; text-decoration: underline; text-underline-offset: 4px; font-weight: 500;">%s</a>`, s.Name, s.Name)
					words[i] = strings.Replace(word, s.Name, linked, 1)
					break
				}
			}
		}
	}
	return strings.Join(words, " ")
}

func renderMarkdownToHTML(md string) string {
	var html strings.Builder
	lines := strings.Split(md, "\n")
	inCodeBlock := false
	inList := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Code block
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

		// Bullet list
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

		// Headings
		if strings.HasPrefix(trimmed, "# ") {
			html.WriteString(fmt.Sprintf("<h1>%s</h1>\n", parseInlineMarkdown(strings.TrimPrefix(trimmed, "# "))))
			continue
		} else if strings.HasPrefix(trimmed, "## ") {
			html.WriteString(fmt.Sprintf("<h2>%s</h2>\n", parseInlineMarkdown(strings.TrimPrefix(trimmed, "## "))))
			continue
		} else if strings.HasPrefix(trimmed, "### ") {
			html.WriteString(fmt.Sprintf("<h3>%s</h3>\n", parseInlineMarkdown(strings.TrimPrefix(trimmed, "### "))))
			continue
		} else if strings.HasPrefix(trimmed, "#### ") {
			html.WriteString(fmt.Sprintf("<h4>%s</h4>\n", parseInlineMarkdown(strings.TrimPrefix(trimmed, "#### "))))
			continue
		}

		// Paragraph
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
	// Escape HTML
	text = strings.ReplaceAll(text, "&", "&amp;")
	text = strings.ReplaceAll(text, "<", "&lt;")
	text = strings.ReplaceAll(text, ">", "&gt;")

	// Inline code: `code`
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

	// Bold: **bold**
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

	// Links: [text](url)
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


