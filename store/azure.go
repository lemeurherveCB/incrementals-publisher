package store

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azfile/fileerror"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azfile/share"
)

func getCredential() (azcore.TokenCredential, error) {
	if key := os.Getenv("AZURE_STORAGE_KEY"); key != "" {
		return nil, fmt.Errorf("shared key credential not supported via TokenCredential interface; use NewClientWithSharedKeyCredential")
	}
	return azidentity.NewDefaultAzureCredential(nil)
}

func getShareClient() (*share.Client, error) {
	account := os.Getenv("AZURE_STORAGE_ACCOUNT")
	shareName := os.Getenv("AZURE_STORAGE_SHARE")
	shareURL := fmt.Sprintf("https://%s.file.core.windows.net/%s", account, shareName)

	if key := os.Getenv("AZURE_STORAGE_KEY"); key != "" {
		cred, err := share.NewSharedKeyCredential(account, key)
		if err != nil {
			return nil, err
		}
		return share.NewClientWithSharedKeyCredential(shareURL, cred, nil)
	}

	cred, err := azidentity.NewDefaultAzureCredential(nil)
	if err != nil {
		return nil, err
	}
	return share.NewClient(shareURL, cred, nil)
}

func putFile(ctx context.Context, shareClient *share.Client, filePath string, content []byte) error {
	parts := strings.Split(filePath, "/")
	fileName := parts[len(parts)-1]
	dirs := parts[:len(parts)-1]

	dirClient := shareClient.NewRootDirectoryClient()
	for _, part := range dirs {
		dirClient = dirClient.NewSubdirectoryClient(part)
		_, err := dirClient.Create(ctx, nil)
		if err != nil && !fileerror.HasCode(err, fileerror.ResourceAlreadyExists) {
			return fmt.Errorf("create directory %q: %w", part, err)
		}
	}

	fileClient := dirClient.NewFileClient(fileName)
	if _, err := fileClient.Create(ctx, int64(len(content)), nil); err != nil {
		return fmt.Errorf("create file %q: %w", fileName, err)
	}
	if _, err := fileClient.UploadRange(ctx, 0, newNopCloser(bytes.NewReader(content)), nil); err != nil {
		return fmt.Errorf("upload file %q: %w", fileName, err)
	}
	return nil
}

func Store(ctx context.Context, jobName, buildID, rawText string) error {
	return StoreWithClient(ctx, jobName, buildID, rawText, nil)
}

func StoreWithClient(ctx context.Context, jobName, buildID, rawText string, shareClient *share.Client) error {
	if shareClient == nil {
		var err error
		shareClient, err = getShareClient()
		if err != nil {
			return fmt.Errorf("get share client: %w", err)
		}
	}
	content := []byte(rawText)
	if err := putFile(ctx, shareClient, fmt.Sprintf("%s/%s.txt", jobName, buildID), content); err != nil {
		return err
	}
	return putFile(ctx, shareClient, fmt.Sprintf("%s/latest.txt", jobName), content)
}

func Probe(ctx context.Context) error {
	shareClient, err := getShareClient()
	if err != nil {
		return err
	}
	_, err = shareClient.GetProperties(ctx, nil)
	return err
}

// nopCloser wraps a *bytes.Reader to satisfy io.ReadSeekCloser.
type nopCloser struct{ *bytes.Reader }

func (nopCloser) Close() error { return nil }

func newNopCloser(r *bytes.Reader) *nopCloser { return &nopCloser{r} }

