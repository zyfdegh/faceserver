package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/davecgh/go-spew/spew"
)

var (
	MaxFileSize = int64(20 * 1024 * 1024)
	// for unix/linux, it's ./public/uploads
	RootDirectory = filepath.Join(".", "public", "uploads")
)

func upload(w http.ResponseWriter, req *http.Request) {
	contentType := req.Header.Get("content-type")
	contentLen := req.ContentLength

	log.Printf("[I] uploading content-type=%s, content-length=%d\n", contentType, contentLen)
	if !strings.Contains(contentType, "multipart/form-data") {
		w.Write([]byte("content-type must be multipart/form-data"))
		return
	}
	if contentLen >= MaxFileSize {
		w.Write([]byte(fmt.Sprintf("[E] file to large, length=%d, limit %d", contentLen, MaxFileSize)))
		return
	}

	err := req.ParseMultipartForm(MaxFileSize)
	if err != nil {
		w.Write([]byte("ParseMultipartForm error:" + err.Error()))
		return
	}
	spew.Dump(req.MultipartForm)

	// 如果指定了 subdir，先创建目录
	var subdir string
	for key, values := range req.MultipartForm.Value {
		log.Printf("[I] multipartform name=%s\n", key)
		if key == "subdir" && len(values) > 0 {
			subdir = req.MultipartForm.Value["subdir"][0]
			dir := filepath.Join(RootDirectory, subdir)
			if err := os.MkdirAll(dir, 0755); err != nil {
				w.Write([]byte("mkdir fail, " + dir))
				return
			}
			continue
		}
	}

	if len(req.MultipartForm.File) == 0 {
		w.Write([]byte("no file"))
		return
	}

	for key, files := range req.MultipartForm.File {
		log.Printf("[I] multipartform name=%s\n", key)

		if len(key) == 0 {
			w.Write([]byte("no multipartform key"))
			return
		}

		if len(files) > 100 {
			w.Write([]byte(fmt.Sprintf("too many files: %d", len(files))))
			return
		}

		var lastErr error
		for _, f := range files {
			fd, err := f.Open()
			if err != nil {
				lastErr = err
				log.Printf("[W] open file error, name=%s, size=%d, err: %s", f.Filename, f.Size, err.Error())
				continue
			}
			if err = writeFile(subdir, f.Filename, fd); err != nil {
				lastErr = err
				continue
			}
			log.Printf("[I] successful uploaded, file=%s, size: %.2f KB\n", f.Filename, float64(contentLen)/1024)
		}
		if lastErr != nil {
			w.Write([]byte("upload failed, last err: " + lastErr.Error()))
			return
		}
		w.Write([]byte(fmt.Sprintf("successful, %d files uploaded", len(files))))
	}
}

func writeFile(subdir, filename string, src io.ReadCloser) error {
	path := filepath.Join(RootDirectory, subdir, filename)
	dst, err := os.Create(path)
	defer dst.Close()
	if err != nil {
		log.Printf("[W] file create error: %v, path=%s\n", err, path)
		return err
	}
	if _, err = io.Copy(dst, src); err != nil {
		log.Printf("[W] file copy error: %v, path=%s\n", err, path)
		return err
	}
	return nil
}

func getContentType(fileName string) (extension, contentType string) {
	arr := strings.Split(fileName, ".")

	// see: https://tool.oschina.net/commons/
	if len(arr) >= 2 {
		extension = arr[len(arr)-1]
		switch strings.ToLower(extension) {
		case "jpeg", "jpe", "jpg":
			contentType = "image/jpeg"
		case "png":
			contentType = "image/png"
		case "gif":
			contentType = "image/gif"
		case "mp4":
			contentType = "video/mpeg4"
		case "mp3":
			contentType = "audio/mp3"
		case "wav":
			contentType = "audio/wav"
		case "pdf":
			contentType = "application/pdf"
		case "doc":
			contentType = "application/msword"
		}
	}
	// .*（ 二进制流，不知道下载文件类型）
	contentType = "application/octet-stream"
	return
}

func download(w http.ResponseWriter, req *http.Request) {
	if req.RequestURI == "/favicon.ico" {
		return
	}

	fmt.Printf("download url=%s \n", req.RequestURI)

	filename := req.RequestURI[1:]
	unescapedURL, err := url.QueryUnescape(filename)
	if err != nil {
		w.Write([]byte(err.Error()))
		return
	}

	f, err := os.Open(filepath.Join(RootDirectory, unescapedURL))
	if err != nil {
		w.WriteHeader(404)
		w.Write([]byte(err.Error()))
		return
	}

	info, err := f.Stat()
	if err != nil {
		w.Write([]byte(err.Error()))
		return
	}

	_, contentType := getContentType(filename)
	w.Header().Set("Content-Disposition", "attachment; filename="+filename)
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Length", strconv.FormatInt(info.Size(), 10))

	f.Seek(0, 0)
	io.Copy(w, f)
}

// 上传请求
// curl -L -X POST 'localhost:8080/file/upload' -H 'Content-Type: multipart/form-data' -F 'file=@"/Users/ferdi/Desktop/Group 886.png"' -F 'subdir="zhao"'
//
// 下载请求
// curl -L -X GET 'localhost:8080/zhao/Group 886.png'

func main() {
	addr := ":8080"

	fmt.Printf("listen on %s...\n", addr)
	http.HandleFunc("/file/upload", upload)
	http.HandleFunc("/", download)
	log.Fatal(http.ListenAndServe(addr, nil))
}
