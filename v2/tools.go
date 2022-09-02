package toolkit

import (
    "bytes"
    "crypto/rand"
    "encoding/json"
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
    MaxFileSize        int
    AllowedFileTypes   []string
    MaxJSONSize        int
    AllowUnknownFields bool
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

// DownloadStaticFile downloads a file, and tries to do not display it in the browser window via content disposition
// It allows specification of the display name
func (t *Tools) DownloadStaticFile(w http.ResponseWriter, r *http.Request, pathName, displayName string) {
    w.Header().Set("Content-Disposition",
        fmt.Sprintf("attachment; filename=\"%s\"", displayName))
    http.ServeFile(w, r, pathName)
}

// JSONResponse is the type used for sending JSON around
type JSONResponse struct {
    Error   bool        `json:"error"`
    Message string      `json:"message"`
    Data    interface{} `json:"data,omitempty"`
}

// ReadJSON tries to read the body of a request and converts it from json to a go data variable
func (t *Tools) ReadJSON(w http.ResponseWriter, r *http.Request, data interface{}) error {
    maxBytes := 1024 * 1024 // 1 MB
    if t.MaxJSONSize != 0 {
        maxBytes = t.MaxJSONSize
    }
    r.Body = http.MaxBytesReader(w, r.Body, int64(maxBytes))
    dec := json.NewDecoder(r.Body)
    if !t.AllowUnknownFields {
        dec.DisallowUnknownFields()
    }
    err := dec.Decode(data)
    if err != nil {
        var (
            syntaxError           *json.SyntaxError
            unmarshalTypeError    *json.UnmarshalTypeError
            invalidUnmarshalError *json.InvalidUnmarshalError
        )
        switch {
        case errors.As(err, &syntaxError):
            return fmt.Errorf("body contains badly-formed JSON (at character %d)",
                syntaxError.Offset)
        case errors.Is(err, io.ErrUnexpectedEOF):
            return errors.New("body contains badly-formed JSON")
        case errors.As(err, &unmarshalTypeError):
            if unmarshalTypeError.Field != "" {
                return fmt.Errorf("body conatians incorrect JSON type for field %q",
                    unmarshalTypeError.Field)
            }
            return fmt.Errorf("body contains incorrect JSON type (at) character %d",
                unmarshalTypeError.Offset)
        case errors.Is(err, io.EOF):
            return errors.New("body must not be empty")
        case strings.HasPrefix(err.Error(), "json: unknown field"):
            fieldName := strings.TrimPrefix(err.Error(), "json: unknown field")
            return fmt.Errorf("body contains unknown key %s", fieldName)
        case err.Error() == "http: request body too large":
            return fmt.Errorf("body must not be large than %d bytes", maxBytes)
        case errors.As(err, &invalidUnmarshalError):
            return fmt.Errorf("error unmarshalling JSON: %s", err.Error())

        default:
            return err
        }
    }
    err = dec.Decode(&struct{}{})
    if err != io.EOF {
        return errors.New("body must contain only one JSON value")
    }
    return nil
}

// WriteJSON takes response status code and arbitrary data and writes json to the client
func (t *Tools) WriteJSON(w http.ResponseWriter, status int, data interface{}, headers ...http.Header) error {
    out, err := json.Marshal(data)
    if err != nil {
        return err
    }

    if len(headers) > 0 {
        for k, v := range headers[0] {
            w.Header()[k] = v
        }
    }

    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(status)
    _, err = w.Write(out)
    if err != nil {
        return err
    }
    return nil
}

// ErrorJSON takes an error and optionally a status code, then generates and sends JSON error message
func (t *Tools) ErrorJSON(w http.ResponseWriter, err error, status ...int) error {
    statusCode := http.StatusBadRequest

    if len(status) > 0 {
        statusCode = status[0]
    }

    var payload JSONResponse
    payload.Error = true
    payload.Message = err.Error()

    return t.WriteJSON(w, statusCode, payload)
}

// PushJSONToRemote posts data to URL as JSON
// returns response, status code, error
// client is optional - if none is specified, we use standard http.Client
func (t *Tools) PushJSONToRemote(uri string, data interface{}, client ...*http.Client) (*http.Response, int, error) {
    // create json
    jsonData, err := json.Marshal(data)
    if err != nil {
        return nil, 0, err
    }

    // check for custom http client
    httpClient := &http.Client{}
    if len(client) > 0 {
        httpClient = client[0]
    }

    // build the request and set the header
    request, err := http.NewRequest("POST", uri, bytes.NewBuffer(jsonData))
    if err != nil {
        return nil, 0, err
    }
    request.Header.Set("Content-Type", "application/json")

    // call remote uri
    response, err := httpClient.Do(request)
    if err != nil {
        return nil, 0, err
    }
    defer response.Body.Close()

    // send response back
    return response, response.StatusCode, nil
}
