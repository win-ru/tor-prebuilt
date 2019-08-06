all:
	make checkdep
	make l
	make d
	make w

checkdep:
	go-bindata -version
	7z i

dep:
	go get -u github.com/PuerkitoBio/goquery
	go get -u github.com/mholt/archiver
	go get -u github.com/cretz/bine
	go get -u github.com/jteeuwen/go-bindata/...

l:
	go run build.go -target linux

d:
	go run build.go -target darwin

w:
	go run build.go -target windows
