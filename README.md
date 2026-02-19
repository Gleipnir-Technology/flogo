# Flogo

`flogo` is a small binary for doing quick edit-compile-test loops with go programs.
It's opninionated and avoids having complex configuration.
There are obviously lots of other tools in this space. Here's what sets `flogo` apart:

| Project | Differences |
| --- | --- |
| [realize](https://github.com/oxequa/realize) | Realize has a complex YAML config, supports multiple projects, and hasn't had a release since 2018
| [air](https://github.com/cosmtrek/air) | Stops binary, then builds, then starts
| [fresh](https://github.com/gravityblast/fresh) | Shows errors in the web browser
| [CompileDaemon](https://github.com/githubnemo/CompileDaemon) | Very simple, watches for file changes and compiles.

 ## Features

  * When it starts it doesn't build if the previous binary is the same
  * Shows build errors in the console, and in the browser
