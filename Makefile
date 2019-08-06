all:
	make checkdep
	make l
	make d
	make w

checkdep:
	go-bindata -version
	7z i

l:
	go run build.go -target linux

d:
	go run build.go -target darwin

w:
	go run build.go -target windows
