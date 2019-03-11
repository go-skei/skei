// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package integrations

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"io/ioutil"
	"net/http"
	"testing"

	"code.skei.dev/skei/models"
	"code.skei.dev/skei/modules/gzip"
	"code.skei.dev/skei/modules/lfs"
	"code.skei.dev/skei/modules/setting"
	"github.com/stretchr/testify/assert"

	gzipp "github.com/klauspost/compress/gzip"
)

func GenerateLFSOid(content io.Reader) (string, error) {
	h := sha256.New()
	if _, err := io.Copy(h, content); err != nil {
		return "", err
	}
	sum := h.Sum(nil)
	return hex.EncodeToString(sum), nil
}

var lfsID = int64(20000)

func storeObjectInRepo(t *testing.T, repositoryID int64, content *[]byte) string {
	oid, err := GenerateLFSOid(bytes.NewReader(*content))
	assert.NoError(t, err)
	var lfsMetaObject *models.LFSMetaObject

	if setting.UsePostgreSQL {
		lfsMetaObject = &models.LFSMetaObject{ID: lfsID, Oid: oid, Size: int64(len(*content)), RepositoryID: repositoryID}
	} else {
		lfsMetaObject = &models.LFSMetaObject{Oid: oid, Size: int64(len(*content)), RepositoryID: repositoryID}
	}

	lfsID = lfsID + 1
	lfsMetaObject, err = models.NewLFSMetaObject(lfsMetaObject)
	assert.NoError(t, err)
	contentStore := &lfs.ContentStore{BasePath: setting.LFS.ContentPath}
	if !contentStore.Exists(lfsMetaObject) {
		err := contentStore.Put(lfsMetaObject, bytes.NewReader(*content))
		assert.NoError(t, err)
	}
	return oid
}

func doLfs(t *testing.T, content *[]byte, expectGzip bool) {
	prepareTestEnv(t)
	repo, err := models.GetRepositoryByOwnerAndName("user2", "repo1")
	assert.NoError(t, err)
	oid := storeObjectInRepo(t, repo.ID, content)
	defer repo.RemoveLFSMetaObjectByOid(oid)

	session := loginUser(t, "user2")

	// Request OID
	req := NewRequest(t, "GET", "/user2/repo1.git/info/lfs/objects/"+oid+"/test")
	req.Header.Set("Accept-Encoding", "gzip")
	resp := session.MakeRequest(t, req, http.StatusOK)

	contentEncoding := resp.Header().Get("Content-Encoding")
	if !expectGzip || !setting.EnableGzip {
		assert.NotContains(t, contentEncoding, "gzip")

		result := resp.Body.Bytes()
		assert.Equal(t, *content, result)
	} else {
		assert.Contains(t, contentEncoding, "gzip")
		gzippReader, err := gzipp.NewReader(resp.Body)
		assert.NoError(t, err)
		result, err := ioutil.ReadAll(gzippReader)
		assert.NoError(t, err)
		assert.Equal(t, *content, result)
	}

}

func TestGetLFSSmall(t *testing.T) {
	content := []byte("A very small file\n")
	doLfs(t, &content, false)
}

func TestGetLFSLarge(t *testing.T) {
	content := make([]byte, gzip.MinSize*10)
	for i := range content {
		content[i] = byte(i % 256)
	}
	doLfs(t, &content, true)
}

func TestGetLFSGzip(t *testing.T) {
	b := make([]byte, gzip.MinSize*10)
	for i := range b {
		b[i] = byte(i % 256)
	}
	outputBuffer := bytes.NewBuffer([]byte{})
	gzippWriter := gzipp.NewWriter(outputBuffer)
	gzippWriter.Write(b)
	gzippWriter.Close()
	content := outputBuffer.Bytes()
	doLfs(t, &content, false)
}

func TestGetLFSZip(t *testing.T) {
	b := make([]byte, gzip.MinSize*10)
	for i := range b {
		b[i] = byte(i % 256)
	}
	outputBuffer := bytes.NewBuffer([]byte{})
	zipWriter := zip.NewWriter(outputBuffer)
	fileWriter, err := zipWriter.Create("default")
	assert.NoError(t, err)
	fileWriter.Write(b)
	zipWriter.Close()
	content := outputBuffer.Bytes()
	doLfs(t, &content, false)
}
