Doc Generator
===

Data driven documentation generator with the a plugin architecture allowing for arbitrary input and output types.

Architecture
---

```
┌────────────────────┐
│   Configuration    │
└────────────────────┘
        │
        ▼
┌────────────────────┐
│  Pipeline Service  │
└────────────────────┘
  │   │        │
  ▼   ▼        ▼
┌───────┐ ┌───────┐ ┌───────┐
│ Input │ │ Build │ │ Output │
│ Plugin│ │ Phase │ │ Plugin │
└───────┘ └───────┘ └───────┘
```

Key Components
---

- Configuration: A JSON file that defines the pipeline.
- Pipeline Service: The main service that orchestrates the pipeline.
- Input Plugin: A plugin that reads the input data.
- Build Phase: The build phase that processes the input data.
- Output Plugin: A plugin that writes the output data.

Input Plugins
---

- Markdown: Markdown files.
- Code: Go files, odin files
- JSON: JSON files.

Inermediate Representation
---

- pkg/store: This package will contain the data structures that represent the input data.
    - The data inside of this should look like an in memory normalized database that can be queried or walked over in order to create outputs
    - Symbols should also have a compatibility table available to them so that for example, things inside of rust stay private to rust, however if a file sits on a rust C ABI, it should be marked that it is compatible with C and can exist in that namespace.
        - In that case, a C plugin could in theory target that function

Output Plugins
---

- Markdown: Markdown files.
- HTML: HTML files.
- PDF: PDF files.

- General outputs, especially for HTML should support queries such as,
    - import graphs
    - caller and calle graphs
    - symbol search
    - module tree
    - symbol compatibility queries.
    - type relationships (inherits / implements / composition)
    - CRAP index
    - large functions
    - TODO index
    - File index
    - Function index
    - struct index
    - struct field index
    - struct method index
    - variable index

- More specific outputs should be produced through a configurable 'audience' tag.
    - i.e. the "API" audience should include function and field docs but exclude macro documentation.
    - the "INTERNAL" audience should include macro documentation.
    - the "USER" should include the documents that an end user of a product would need to know.
    - the "DEVELOPER" should include the documents that a developer of the product would need to know.
    - these tags must all be user defined and used in documentation in the codebase, but this would allow all documentation to live close to the code.

Build Phases
---

- Parse: Parse the input data.
- Process: Process the input data.
- Generate: Generate the output data.