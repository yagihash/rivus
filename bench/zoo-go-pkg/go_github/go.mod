module zoopkg/go_github

go 1.27

require (
	github.com/google/go-github/v93 v93.0.0
	zoo v0.0.0
)

replace zoo => ../../zoo-go

replace github.com/google/go-github/v93 => ./skel
