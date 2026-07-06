package proofstorage

import (
	"context"
	"errors"
	"fmt"
)

var ErrUnavailableProof = errors.New("proof media could not be persisted")

type Store interface {
	PersistFromURL(ctx context.Context, submissionID int64, sourceURL string) (proofURL string, err error)
	DeleteBySubmissionID(ctx context.Context, submissionID int64) error
}

func objectKey(submissionID int64, ext string) string {
	return fmt.Sprintf("%d%s", submissionID, ext)
}

func buildPublicURL(baseURL, key string) string {
	if baseURL == "" {
		return key
	}
	for len(baseURL) > 0 && baseURL[len(baseURL)-1] == '/' {
		baseURL = baseURL[:len(baseURL)-1]
	}
	return baseURL + "/" + key
}
