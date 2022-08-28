package toolkit

import (
    "fmt"
    "image"
    "image/png"
    "io"
    "mime/multipart"
    "net/http/httptest"
    "os"
    "sync"
    "testing"
)

func TestTools_RandomString(t *testing.T) {
    var testTools Tools
    const l = 10

    s := testTools.RandomString(l)
    if len(s) != l {
        t.Error("wrong length string returned")
    }
}

var uploadTests = []struct {
    name          string
    allowedTypes  []string
    renameFile    bool
    errorExpected bool
}{
    {name: "allowed and no rename", allowedTypes: []string{"image/jpeg", "image/png"}, renameFile: false, errorExpected: false},
    {name: "allowed and rename", allowedTypes: []string{"image/jpeg", "image/png"}, renameFile: true, errorExpected: false},
    {name: "not allowed", allowedTypes: []string{"image/jpeg"}, renameFile: false, errorExpected: true},
}

func TestTools_UploadFiles(t *testing.T) {
    for _, test := range uploadTests {

        // set up a pipe to avoid buffering
        pr, pw := io.Pipe()
        writer := multipart.NewWriter(pw)

        wg := sync.WaitGroup{}
        wg.Add(1)
        go func() {
            defer writer.Close()
            defer wg.Done()

            /// create the form data field 'file'
            part, err := writer.CreateFormFile("file", "./tesdata/sample1.png")
            if err != nil {
                t.Error(err)
            }

            f, err := os.Open("./testdata/sample1.png")
            if err != nil {
                t.Error(err)
            }
            defer f.Close()

            img, _, err := image.Decode(f)
            if err != nil {
                t.Error("error decoding image", err)
            }
            err = png.Encode(part, img)
            if err != nil {
                t.Error(err)
            }
        }()

        // read form pipe which receives data
        request := httptest.NewRequest("POST", "/", pr)
        request.Header.Add("Content-Type", writer.FormDataContentType())

        var testTools Tools
        testTools.AllowedFileTypes = test.allowedTypes

        UploadedFiles, err := testTools.UploadFiles(request, "./testdata/uploads", test.renameFile)
        if err != nil && !test.errorExpected {
            t.Error(err)
        }
        if !test.errorExpected {
            if _, err = os.Stat(fmt.Sprintf("./testdata/uploads/%s", UploadedFiles[0].NewFileName)); os.IsNotExist(err) {
                t.Errorf("%s expected file to exist: %s", test.name, err.Error())
            }
            // clean up
            _ = os.Remove(fmt.Sprintf("./testdata/uploads/%s", UploadedFiles[0].NewFileName))
        }

        if !test.errorExpected && err != nil {
            t.Errorf("%s error expected but none received", test.name)
        }

        wg.Wait()
    }
}

func TestTools_UploadFile(t *testing.T) {
    pr, pw := io.Pipe()
    writer := multipart.NewWriter(pw)

    go func() {
        defer writer.Close()

        /// create the form data field 'file'
        part, err := writer.CreateFormFile("file", "./tesdata/sample1.png")
        if err != nil {
            t.Error(err)
        }

        f, err := os.Open("./testdata/sample1.png")
        if err != nil {
            t.Error(err)
        }
        defer f.Close()

        img, _, err := image.Decode(f)
        if err != nil {
            t.Error("error decoding image", err)
        }
        err = png.Encode(part, img)
        if err != nil {
            t.Error(err)
        }
    }()

    // read form pipe which receives data
    request := httptest.NewRequest("POST", "/", pr)
    request.Header.Add("Content-Type", writer.FormDataContentType())

    var testTools Tools

    UploadedFiles, err := testTools.UploadFile(request, "./testdata/uploads", true)
    if err != nil {
        t.Error(err)
    }
    if _, err = os.Stat(fmt.Sprintf("./testdata/uploads/%s", UploadedFiles.NewFileName)); os.IsNotExist(err) {
        t.Errorf("expected file to exist: %s", err.Error())
    }
    // clean up
    _ = os.Remove(fmt.Sprintf("./testdata/uploads/%s", UploadedFiles.NewFileName))

}

func TestTools_CreateDirIfNotExist(t *testing.T) {
    var testTool Tools
    err := testTool.CreateDirIfNotExist("./testdata/myDir")
    if err != nil {
        t.Error(err)
    }

    err = testTool.CreateDirIfNotExist("./testdata/myDir")
    if err != nil {
        t.Error(err)
    }

    _ = os.Remove("./testdata/myDir")
}

var slugTests = []struct {
    name          string
    s             string
    expected      string
    errorExpected bool
}{
    {name: "valid string", s: "now is the time", expected: "now-is-the-time", errorExpected: false},
    {name: "empty string", s: "", expected: "", errorExpected: true},
    {name: "japanese string", s: "こんにちは世界", expected: "", errorExpected: true},
    {name: "japanese string and latin", s: "こんにちは世界 hello world", expected: "hello-world", errorExpected: false},
    {
        name:          "complex string",
        s:             "Now is the time for all GOOD men!!! + fish & such *()^13",
        expected:      "now-is-the-time-for-all-good-men-fish-such-13",
        errorExpected: false,
    },
}

func TestTools_Slugify(t *testing.T) {
    var testTool Tools
    for _, test := range slugTests {
        slug, err := testTool.Slugify(test.s)
        if err != nil && !test.errorExpected {
            t.Errorf("%s: error received when none expected: %s", test.name, err.Error())
        }
        if !test.errorExpected && slug != test.expected {
            t.Errorf("%s: wrong slug returned; expected: %s, received: %s", test.name, test.expected, slug)
        }
    }
}
