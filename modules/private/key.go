// Copyright 2018 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package private

import (
	"encoding/json"
	"fmt"

	"code.skei.dev/skei/models"
	"code.skei.dev/skei/modules/log"
	"code.skei.dev/skei/modules/setting"
)

// UpdateDeployKeyUpdated update deploy key updates
func UpdateDeployKeyUpdated(keyID int64, repoID int64) error {
	reqURL := setting.LocalURL + fmt.Sprintf("api/internal/repositories/%d/keys/%d/update", repoID, keyID)
	log.GitLogger.Trace("UpdateDeployKeyUpdated: %s", reqURL)

	resp, err := newInternalRequest(reqURL, "POST").Response()
	if err != nil {
		return err
	}

	defer resp.Body.Close()

	// All 2XX status codes are accepted and others will return an error
	if resp.StatusCode/100 != 2 {
		return fmt.Errorf("Failed to update deploy key: %s", decodeJSONError(resp).Err)
	}
	return nil
}

// HasDeployKey check if repo has deploy key
func HasDeployKey(keyID, repoID int64) (bool, error) {
	reqURL := setting.LocalURL + fmt.Sprintf("api/internal/repositories/%d/has-keys/%d", repoID, keyID)
	log.GitLogger.Trace("HasDeployKey: %s", reqURL)

	resp, err := newInternalRequest(reqURL, "GET").Response()
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == 200 {
		return true, nil
	}
	return false, nil
}

// GetPublicKeyByID  get public ssh key by his ID
func GetPublicKeyByID(keyID int64) (*models.PublicKey, error) {
	reqURL := setting.LocalURL + fmt.Sprintf("api/internal/ssh/%d", keyID)
	log.GitLogger.Trace("GetPublicKeyByID: %s", reqURL)

	resp, err := newInternalRequest(reqURL, "GET").Response()
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("Failed to get repository: %s", decodeJSONError(resp).Err)
	}

	var pKey models.PublicKey
	if err := json.NewDecoder(resp.Body).Decode(&pKey); err != nil {
		return nil, err
	}
	return &pKey, nil
}

// GetUserByKeyID get user attached to key
func GetUserByKeyID(keyID int64) (*models.User, error) {
	reqURL := setting.LocalURL + fmt.Sprintf("api/internal/ssh/%d/user", keyID)
	log.GitLogger.Trace("GetUserByKeyID: %s", reqURL)

	resp, err := newInternalRequest(reqURL, "GET").Response()
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("Failed to get user: %s", decodeJSONError(resp).Err)
	}

	var user models.User
	if err := json.NewDecoder(resp.Body).Decode(&user); err != nil {
		return nil, err
	}

	return &user, nil
}

// UpdatePublicKeyUpdated update public key updates
func UpdatePublicKeyUpdated(keyID int64) error {
	// Ask for running deliver hook and test pull request tasks.
	reqURL := setting.LocalURL + fmt.Sprintf("api/internal/ssh/%d/update", keyID)
	log.GitLogger.Trace("UpdatePublicKeyUpdated: %s", reqURL)

	resp, err := newInternalRequest(reqURL, "POST").Response()
	if err != nil {
		return err
	}

	defer resp.Body.Close()

	// All 2XX status codes are accepted and others will return an error
	if resp.StatusCode/100 != 2 {
		return fmt.Errorf("Failed to update public key: %s", decodeJSONError(resp).Err)
	}
	return nil
}
