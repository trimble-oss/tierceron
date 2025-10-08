Follow these steps to create a new ETL test driven project.

1. Copy this directory as a seed folder to a new project folder at the same level as ETL.Core (for example ETL.MyProject)

2. In your new project folder, rename vault_templates/ETL/Core to vault_templates/ETL/MyProject.  Note, if you changed ETL to something else, you should also change the ETL portion of the path.

3. If you want new model objects to interact more easily with MS SQL database, go to the parent of ETL.MyProject and run:
go get -u golang.org/x/tools/cmd/goimports
go get -u github.com/xo/xo

NOTE: If running Windows, you will need to install xo on WSL (tested on Debian) and generate your models there.  For some reason, the project does *not* compile on Windows.

4. In ETL.MyProject, run xo, pointing it at your SQL database to generate model objects to work with.

Examples:

Clean everything:
go test -clean
Clean only one test:
go test -run .*ItemAddon -clean

Clean by environment other than dev (Order of parameters matters):
go test -run AddItemAddon -clean -env=QA

Run everything in parallel:
go test -v -parallel 100

Run set of tests
go test -v -parallel 10 -run .*ItemAddon

Run set of benchmarks (-run= with some non-matching string is required to prevent tests from also running)
go test -v -bench=.*ItemAddon -run=XX

Clean test cache
go clean -testcache