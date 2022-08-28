package toolkit

import (
    "crypto/rand"
    "errors"
    "fmt"
    "io"
    "math/big"
    "mime/multipart"
    "net/http"
    "os"
    "path/filepath"
    "regexp"
    "strings"
)

const randomStringSource = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789_+"

// Tools is a type used to instantiate this module. Any variable of this type will have access
// to all the methods with the receiver *Tools
type Tools struct {
    MaxFileSize      int
    AllowedFileTypes []string
}

// RandomString returns a string of random characters of length n, using randomStringSource
// as a source of characters to generate random string
func (t *Tools) RandomString(n int) string {
    var (
        p    *big.Int
        x, y uint64
    )
    s, r := make([]rune, n), []rune(randomStringSource)
    for i := range s {
        p, _ = rand.Prime(rand.Reader, len(r))
        x, y = p.Uint64(), uint64(len(r))
        s[i] = r[x%y]
    }
    return string(s)
}

// UploadedFile is a type used to save information about an uploaded file
type UploadedFile struct {
    NewFileName      string
    OriginalFileName string
    FileSize         int64
}

func (t *Tools) UploadFile(r *http.Request, uploadDir string, rename ...bool) (*UploadedFile, error) {
    renameFile := true
    if len(rename) > 0 {
        renameFile = rename[0]
    }
    files, err := t.UploadFiles(r, uploadDir, renameFile)
    if err != nil {
        return nil, err
    }
    return files[0], err
}

// UploadFiles upload one or more files to the specifies directory and gives to files random names
// It returns slice of newly named files, the original names, the size and potential error
func (t *Tools) UploadFiles(r *http.Request, uploadDir string, rename ...bool) ([]*UploadedFile, error) {
    renameFile := true
    if len(rename) > 0 {
        renameFile = rename[0]
    }

    var (
        uploadedFiles []*UploadedFile
    )

    if t.MaxFileSize == 0 {
        t.MaxFileSize = 1024 * 1024 * 1024 // 1 GB
    }

    err := t.CreateDirIfNotExist(uploadDir)
    if err != nil {
        return nil, err
    }

    err = r.ParseMultipartForm(int64(t.MaxFileSize))
    if err != nil {
        return nil, errors.New("the uploaded file is too big")
    }

    for _, headers := range r.MultipartForm.File {
        for _, header := range headers {
            uploadedFiles, err = func(uploadedFiles []*UploadedFile) ([]*UploadedFile, error) {
                var (
                    uploadedFile UploadedFile
                    infile       multipart.File
                )
                infile, err = header.Open()
                if err != nil {
                    return nil, err
                }
                defer infile.Close()
                buff := make([]byte, 512)
                _, err = infile.Read(buff)
                if err != nil {
                    return nil, err
                }

                // check if the file type is permitted
                allowed := false
                fileType := http.DetectContentType(buff)

                if len(t.AllowedFileTypes) > 0 {
                    for _, x := range t.AllowedFileTypes {
                        if strings.EqualFold(fileType, x) {
                            allowed = true
                        }
                    }
                } else {
                    allowed = true
                }

                if !allowed {
                    if err != nil {
                        return nil, errors.New("type file is not allowed to upload")
                    }
                }

                _, err = infile.Seek(0, 0)
                if err != nil {
                    return nil, err
                }

                if renameFile {
                    uploadedFile.NewFileName = fmt.Sprintf("%s%s",
                        t.RandomString(25), filepath.Ext(header.Filename))
                } else {
                    uploadedFile.NewFileName = header.Filename
                }

                uploadedFile.OriginalFileName = header.Filename

                var (
                    outfile  *os.File
                    fileSize int64
                )
                defer outfile.Close()

                if outfile, err = os.Create(filepath.Join(uploadDir, uploadedFile.NewFileName)); err != nil {
                    return nil, err
                } else {
                    fileSize, err = io.Copy(outfile, infile)
                    if err != nil {
                        return nil, err
                    }
                    uploadedFile.FileSize = fileSize
                }
                uploadedFiles = append(uploadedFiles, &uploadedFile)
                return uploadedFiles, nil
            }(uploadedFiles)
            if err != nil {
                return uploadedFiles, err
            }
        }
    }
    return uploadedFiles, nil
}

// CreateDirIfNotExist creates a directory and all necessary parents, if it does not exist
func (t *Tools) CreateDirIfNotExist(path string) error {
    const mode = 0755
    var err error
    if _, err = os.Stat(path); os.IsNotExist(err) {
        err = os.MkdirAll(path, mode)
        if err != nil {
            return err
        }
    }
    return nil
}

// Slugify creates slug from a string
func (t *Tools) Slugify(s string) (string, error) {
    if s == "" {
        return "", errors.New("empty string is not permitted")
    }

    re := regexp.MustCompile(`[^a-z\d]+`)
    slug := strings.Trim(re.ReplaceAllString(strings.ToLower(s), "-"), "-")
    if len(slug) == 0 {
        return "", errors.New("after removing characters, slug is zero length")
    }

    return slug, nil
}
