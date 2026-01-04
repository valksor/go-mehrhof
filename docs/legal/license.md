# License

Valksor Mehrhof is licensed under the **BSD 3-Clause License**.

## BSD 3-Clause License

Copyright (c) 2025+, Dāvis Zālītis (k0d3r1s)  
Copyright (c) 2025+, SIA Valksor

Redistribution and use in source and binary forms, with or without
modification, are permitted provided that the following conditions are met:

1. Redistributions of source code must retain the above copyright notice, this
list of conditions and the following disclaimer.

2. Redistributions in binary form must reproduce the above copyright notice,
this list of conditions and the following disclaimer in the documentation
and/or other materials provided with the distribution.

3. Neither the name of the copyright holder nor the names of its
contributors may be used to endorse or promote products derived from
this software without specific prior written permission.

THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS "AS IS"
AND ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT LIMITED TO, THE
IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR A PARTICULAR PURPOSE ARE
DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT HOLDER OR CONTRIBUTORS BE LIABLE
FOR ANY DIRECT, INDIRECT, INCIDENTAL, SPECIAL, EXEMPLARY, OR CONSEQUENTIAL
DAMAGES (INCLUDING, BUT NOT LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS OR
SERVICES; LOSS OF USE, DATA, OR PROFITS; OR BUSINESS INTERRUPTION) HOWEVER
CAUSED AND ON ANY THEORY OF LIABILITY, WHETHER IN CONTRACT, STRICT LIABILITY,
OR TORT (INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF THE USE
OF THIS SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.

## Third-Party Licenses

Mehrhof uses third-party Go modules, each with their own licenses. You can view
the full list of dependencies and their licenses by running:

```bash
go mod download
go list -m all | xargs -I {} sh -c 'echo "{}:" && go mod download -json {} | jq -r ".License"'
```

Or check the `go.sum` file in the repository root for exact dependency versions.

## What You Can Do

Under this license, you are free to:

- ✓ Use Mehrhof in commercial and personal projects
- ✓ Modify the source code for your needs
- ✓ Distribute modified versions (source or binary)
- ✓ Sublicense the code

## Requirements

When redistributing, you must:

1. Include the copyright notice and license text
2. State any changes you made to the original code
3. Not use the copyright holder's names to endorse your product

## Full License Text

The complete license text is also available in the [LICENSE](https://github.com/valksor/go-mehrhof/blob/master/LICENSE) file
in the repository root.
