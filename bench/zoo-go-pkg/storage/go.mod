module zoopkg/storage

go 1.27

require (
	cloud.google.com/go/storage v1.71.0
	zoo v0.0.0
)

replace zoo => ../../zoo-go

replace cloud.google.com/go/storage => ./skel
