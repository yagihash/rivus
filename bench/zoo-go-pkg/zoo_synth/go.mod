module zoopkg/zoo_synth

go 1.27

require (
	example.invalid/zoosynth v0.0.0
	zoo v0.0.0
)

replace zoo => ../../zoo-go

replace example.invalid/zoosynth => ./skel
