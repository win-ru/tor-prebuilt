package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/clementauger/tor-prebuilt/embedded"
	"github.com/mholt/archiver"
)

func main() {
	if err := mmain(); err != nil {
		log.Fatal(err)
	}
}

var (
	linux   = "linux"
	darwin  = "darwin"
	windows = "windows"
)

func mmain() error {

	var target string
	flag.StringVar(&target, "target", linux, "OS target (linux/windows/darwin)")
	flag.Parse()

	dldir := ".download"
	outdir := "out"
	{
		os.MkdirAll(dldir, os.ModePerm)
		os.MkdirAll(outdir, os.ModePerm)
	}
	distDir, err := fetchTor(outdir, dldir, target)
	if err != nil {
		return err
	}
	pkgName, err := genBinData(distDir, target)
	if err != nil {
		return err
	}
	pkgDir := filepath.Join("embedded", pkgName)
	latest := filepath.Join("embedded", "tor_latest")
	os.RemoveAll(latest)
	os.MkdirAll(latest, os.ModePerm)
	return copyRecursive(latest, pkgDir)
}

func genBinData(distDir, target string) (string, error) {

	ver := versionR.FindString(distDir)
	if ver == "" {
		return "", fmt.Errorf("version not found in package directory %q for target=%q", distDir, target)
	}

	pkgName := "tor_" + strings.Replace(ver, ".", "_", -1)
	os.MkdirAll(filepath.Join("embedded", pkgName), os.ModePerm)

	assets := filepath.Join("embedded", pkgName, target+".go")
	log.Println("generating assets ", assets)

	cmd := exec.Command("go-bindata", "-nomemcopy", "-pkg", pkgName, "-prefix", distDir, "-tags", target, "-o", assets, distDir)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return pkgName, cmd.Run()
}

var urlBase = "https://www.torproject.org"

func fetchTor(outdir, dldir, target string) (string, error) {

	if target == linux || target == darwin {

		var body bytes.Buffer
		{
			res, err := http.Get(urlBase + "/download/")
			if err != nil {
				return "", err
			}
			defer res.Body.Close()
			_, err = io.Copy(&body, res.Body)
			if err != nil {
				return "", err
			}
		}

		var archiveURL string
		{
			doc, err := goquery.NewDocumentFromReader(&body)
			if err != nil {
				return "", err
			}
			doc.Find(".downloadLink").Each(func(i int, s *goquery.Selection) {
				href, _ := s.Attr("href")
				if strings.Contains(href, "tar.xz") && target == linux {
					archiveURL = href
				} else if strings.Contains(href, "dmg") && target == darwin {
					archiveURL = href
				}
			})
		}
		if archiveURL == "" {
			return "", fmt.Errorf("download link not found for target=%q", target)
		}
		if !strings.HasPrefix(archiveURL, "http://") && !strings.HasPrefix(archiveURL, "https://") {
			archiveURL = urlBase + archiveURL
		}
		log.Println("proceeding with ", archiveURL)

		ver := versionR.FindString(archiveURL)
		if ver == "" {
			return "", fmt.Errorf("version not found in download link url %q for target=%q", archiveURL, target)
		}

		archiveName := filepath.Base(archiveURL)
		archivePath := filepath.Join(dldir, archiveName)
		archivePath, err := filepath.Abs(archivePath)
		if err != nil {
			return "", err
		}

		if err := dlFile(archivePath, archiveURL); err != nil {
			return "", err
		}

		tmpDir, err := ioutil.TempDir("", "")
		if err != nil {
			return "", err
		}
		defer os.RemoveAll(tmpDir)

		pkgOutDir := filepath.Join(outdir, target, ver)
		os.RemoveAll(pkgOutDir)
		os.MkdirAll(pkgOutDir, os.ModePerm)

		if target == linux {
			if err := archiver.Unarchive(archivePath, tmpDir); err != nil {
				return "", err
			}

			srcPkgDir := filepath.Join(tmpDir, "tor-browser_en-US/Browser/TorBrowser/Tor/")
			if err := copyRecursive(pkgOutDir, srcPkgDir); err != nil {
				return "", err
			}
			srcPkgDir = filepath.Join(tmpDir, "tor-browser_en-US/Browser/TorBrowser/Data/Tor/")
			if err := copyRecursive(pkgOutDir, srcPkgDir); err != nil {
				return "", err
			}

			for _, file := range []string{filepath.Join(pkgOutDir, "torrc-defaults"), filepath.Join(pkgOutDir, "torrc")} {
				b, err := ioutil.ReadFile(file)
				if err != nil {
					continue
				}
				b = bytes.Replace(b, []byte("./TorBrowser/Tor/"), []byte("./"), -1)
				err = ioutil.WriteFile(file, b, os.ModePerm)
				if err != nil {
					return "", err
				}
			}

			{
				torrc := filepath.Join(pkgOutDir, "torrc-defaults")
				if _, err := os.Stat(torrc); os.IsNotExist(err) {
					if err := ioutil.WriteFile(torrc, []byte{}, os.ModePerm); err != nil {
						return "", err
					}
				}
			}

			{
				torrc := filepath.Join(pkgOutDir, "torrc")
				if _, err := os.Stat(torrc); os.IsNotExist(err) {
					if err := ioutil.WriteFile(torrc, []byte{}, os.ModePerm); err != nil {
						return "", err
					}
				}
			}

			return pkgOutDir, nil

		} else if target == darwin {
			cmd := exec.Command("7z", "x", archivePath)
			cmd.Dir = tmpDir
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			if err := cmd.Run(); err != nil {
				return "", err
			}

			srcPkgDir := filepath.Join(tmpDir, "Tor Browser.app/Contents/Resources/TorBrowser/Tor/")
			if err := copyRecursive(pkgOutDir, srcPkgDir); err != nil {
				return "", err
			}

			srcPkgDir = filepath.Join(tmpDir, "Tor Browser.app/Contents/MacOS/Tor/")
			if err := copyRecursive(pkgOutDir, srcPkgDir); err != nil {
				return "", err
			}
			os.Remove(filepath.Join(pkgOutDir, "tor"))
			os.Rename(filepath.Join(pkgOutDir, "tor.real"), filepath.Join(pkgOutDir, "tor"))

			{
				torrc := filepath.Join(pkgOutDir, "torrc-defaults")
				if _, err := os.Stat(torrc); os.IsNotExist(err) {
					if err := ioutil.WriteFile(torrc, []byte{}, os.ModePerm); err != nil {
						return "", err
					}
				}
			}

			{
				torrc := filepath.Join(pkgOutDir, "torrc")
				if _, err := os.Stat(torrc); os.IsNotExist(err) {
					if err := ioutil.WriteFile(torrc, []byte{}, os.ModePerm); err != nil {
						return "", err
					}
				}
			}

			return pkgOutDir, nil
		}

	}

	if target == windows {
		var body bytes.Buffer
		{
			res, err := http.Get(urlBase + "/download/tor/")
			if err != nil {
				return "", err
			}
			defer res.Body.Close()
			_, err = io.Copy(&body, res.Body)
			if err != nil {
				return "", err
			}
		}

		var archiveURL string
		{
			doc, err := goquery.NewDocumentFromReader(&body)
			if err != nil {
				return "", err
			}
			doc.Find(".downloadLink").Each(func(i int, s *goquery.Selection) {
				href, _ := s.Attr("href")
				if strings.Contains(href, "win") {
					archiveURL = href
				}
			})
		}
		if archiveURL == "" {
			return "", fmt.Errorf("download link not found for target=%q", target)
		}
		if !strings.HasPrefix(archiveURL, "http://") && !strings.HasPrefix(archiveURL, "https://") {
			archiveURL = urlBase + archiveURL
		}
		log.Println("proceeding with ", archiveURL)

		ver := versionR.FindString(archiveURL)
		if ver == "" {
			return "", fmt.Errorf("version not found in download link url %q for target=%q", archiveURL, target)
		}

		archiveName := filepath.Base(archiveURL)
		archivePath := filepath.Join(dldir, archiveName)
		archivePath, err := filepath.Abs(archivePath)
		if err != nil {
			return "", err
		}

		if err := dlFile(archivePath, archiveURL); err != nil {
			return "", err
		}

		tmpDir, err := ioutil.TempDir("", "")
		if err != nil {
			return "", err
		}
		defer os.RemoveAll(tmpDir)

		pkgOutDir := filepath.Join(outdir, target, ver)
		os.RemoveAll(pkgOutDir)
		os.MkdirAll(pkgOutDir, os.ModePerm)

		if err := archiver.Unarchive(archivePath, tmpDir); err != nil {
			return "", err
		}

		srcPkgDir := filepath.Join(tmpDir, "Tor/")
		if err := copyRecursive(pkgOutDir, srcPkgDir); err != nil {
			return "", err
		}

		srcPkgDir = filepath.Join(tmpDir, "Data/Tor/")
		if err := copyRecursive(pkgOutDir, srcPkgDir); err != nil {
			return "", err
		}

		{
			torrc := filepath.Join(pkgOutDir, "torrc-defaults")
			if _, err := os.Stat(torrc); os.IsNotExist(err) {
				data := []byte(embedded.TorRCDefaults)
				if err := ioutil.WriteFile(torrc, data, os.ModePerm); err != nil {
					return "", err
				}
			}
		}

		{
			torrc := filepath.Join(pkgOutDir, "torrc")
			if _, err := os.Stat(torrc); os.IsNotExist(err) {
				if err := ioutil.WriteFile(torrc, []byte{}, os.ModePerm); err != nil {
					return "", err
				}
			}
		}

		return pkgOutDir, nil

	}

	return "", fmt.Errorf("unknown target %q", target)
}

var versionR = regexp.MustCompile(`([0-9]+\.[0-9]+\.[0-9]+)`)

func copyRecursive(dst, src string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if path == src {
			return nil
		}
		r, _ := filepath.Rel(src, path)
		r = filepath.Join(dst, r)
		if info.IsDir() {
			return os.MkdirAll(r, os.ModePerm)
		}
		f, err := os.Open(path)
		if err != nil {
			return err
		}
		defer f.Close()
		fo, err := os.Create(r)
		if err != nil {
			return err
		}
		defer fo.Close()
		_, err = io.Copy(fo, f)
		return err
	})
}

func dlFile(dst, srcURL string) error {
	if _, err := os.Stat(dst); os.IsNotExist(err) {
		res, err := http.Get(srcURL)
		if err != nil {
			return err
		}
		defer res.Body.Close()
		f, err := os.Create(dst)
		if err != nil {
			return err
		}
		io.Copy(f, res.Body)
		f.Close()
		//todo signature test
		// gpg --keyserver pool.sks-keyservers.net --recv-keys 0x4E2C6E8793298290
		// gpg --fingerprint 0x4E2C6E8793298290
		// gpg --verify tor-browser-linux64-8.0.8_en-US.tar.xz.asc
	}
	return nil
}
