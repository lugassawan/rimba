---
title: rimba completion
parent: Command
nav_order: 24
---

# rimba completion

Generate shell completion scripts for bash, zsh, fish, or PowerShell.

## Synopsis

```sh
rimba completion <shell>
```

Supported shells: `bash`, `zsh`, `fish`, `powershell`.

## Examples

```sh
rimba completion bash        # Generate bash completions
rimba completion zsh         # Generate zsh completions
rimba completion fish        # Generate fish completions
rimba completion powershell  # Generate PowerShell completions
```

## Common workflows

**Install completions for zsh**
```sh
rimba completion zsh > "${fpath[1]}/_rimba"
# Then restart your shell or run: autoload -U compinit && compinit
```

**Install completions for bash**
```sh
rimba completion bash > /etc/bash_completion.d/rimba
# Or for a single user:
rimba completion bash >> ~/.bash_completion
```

**Install completions for fish**
```sh
rimba completion fish > ~/.config/fish/completions/rimba.fish
```

{: .note }
> Follow the printed instructions after generating to install the completions for your shell.

## Related commands

- [rimba version](version) · check your rimba version
- [rimba update](update) · upgrade to the latest release
