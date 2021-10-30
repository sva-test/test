// Package p contains a Pub/Sub Cloud Function.
package p

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
	"strings"
	"sync"

	"cloud.google.com/go/storage"
	"google.golang.org/api/iterator"

	"github.com/souvenirapps/ide-github-worker/error"
	log "github.com/souvenirapps/ide-github-worker/logger"
)

var once = &sync.Once{}

var storageClient *storage.Client

var (
	bucketName     = os.Getenv("STORAGE_BUCKET_NAME")
	committerEmail = os.Getenv("GIT_COMMITTER_EMAIL")
	committerName  = os.Getenv("GIT_COMMITTER_NAME")
)

// PubSubMessage is the payload of a Pub/Sub event. Please refer to the docs for
// additional information regarding Pub/Sub events.
type PubSubMessage struct {
	Data []byte `json:"data"`
}

// Data is the payload that a message data from Pub/Sub is expected to contain.
type Data struct {
	// UserID is the unique user identifier in IDE service.
	UserID string `json:"user_id"`
	// GithubToken is the GitHub oauth access token of the user used to clone and commit to GitHub repo.
	GithubToken string `json:"github_token"`
	// GithubUsername is the username of the user on GitHub.
	GithubUsername string `json:"github_username"`
	// RepoName is the name of the user's GitHub repo.
	RepoName string `json:"repo_name"`
	// ProjectName is the name of the folder in user's GitHub repo.
	ProjectName string `json:"project_name"`
	// FileName is the name of the file in user's GitHub repo.
	FileName string `json:"file_name"`
	// StorageFilePath is the path in storage bucket from where the file has to be read.
	// The contents of this file is pushed to user's GitHub repo.
	StorageFilePath string `json:"storage_file_path"`
	// CommitMessage is the message used in Git commit.
	CommitMessage string `json:"commit_message"`
}

// Run consumes a Pub/Sub message.
func Run(ctx context.Context, m PubSubMessage) (err error) {
	data := new(Data)
	if err = json.Unmarshal(m.Data, data); err != nil {
		log.Error(err)
		return
	}

	once.Do(func() { err = setup(ctx) })
	if err != nil {
		log.Error(err)
		return
	}

	if err = process(ctx, data); err != nil {
		log.Error(err)
	}

	return
}

func setup(ctx context.Context) error {
	var err error
	if storageClient, err = storage.NewClient(ctx); err != nil {
		return err
	}

	cmd := exec.CommandContext(ctx, "git",
		"config", "--global", "user.email", fmt.Sprintf("\"%s\"", committerEmail))
	if out, err := cmd.Output(); err != nil {
		return worker_error.New(err, string(out))
	}
	cmd = exec.CommandContext(ctx, "git",
		"config", "--global", "user.name", fmt.Sprintf("\"%s\"", committerName))
	if out, err := cmd.Output(); err != nil {
		return worker_error.New(err, string(out))
	}

	return nil
}

func process(ctx context.Context, data *Data) error {
	defer data.cleanup()

	if err := data.makeWorkingDir(ctx); err != nil {
		return worker_error.New(err, "makeWorkingDir")
	}
	if err := data.cloneGithubRepo(ctx); err != nil {
		return worker_error.New(err, "cloneGithubRepo")
	}
	if err := data.writeContentFromStorageToFile(ctx); err != nil {
		return worker_error.New(err, "writeContentFromStorageToFile")
	}
	if err := data.makeGitCommit(ctx); err != nil {
		return worker_error.New(err, "makeGitCommit")
	}
	if err := data.pushToRemote(ctx); err != nil {
		return worker_error.New(err, "pushToRemote")
	}

	return nil
}

func (d *Data) makeWorkingDir(_ context.Context) error {
	return os.Mkdir(d.localWorkingDir(), os.ModePerm)
}

func (d *Data) cloneGithubRepo(ctx context.Context) error {
	cmd := exec.CommandContext(ctx, "git",
		"clone", fmt.Sprintf("https://%s@github.com/%s/%s.git", d.GithubToken, d.GithubUsername, d.RepoName))
	cmd.Dir = d.localWorkingDir()
	if out, err := cmd.Output(); err != nil {
		return worker_error.New(err, string(out))
	}

	return nil
}

func (d *Data) writeContentFromStorageToFile(ctx context.Context) error {
	bucket := storageClient.Bucket(bucketName)
	it := bucket.Objects(ctx, &storage.Query{
		Prefix: d.UserID,
	})
	files := make([]string, 0, 10)
	for {
		attrs, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			log.Error(fmt.Sprintf("Bucket(%q).Objects: %v", bucketName, err))
			return err
		}
		name := attrs.Name
		if !strings.HasSuffix(name, "/") {
			files = append(files, name)
		}
	}

	for _, file := range files {
		if err := d._writeContentFromStorageToFile(ctx, bucket, file); err != nil {
			return err
		}
	}

	return nil
}

func (d *Data) _writeContentFromStorageToFile(ctx context.Context, bucket *storage.BucketHandle, object string) (err error) {
	r, err := bucket.Object(object).NewReader(ctx)
	if err != nil {
		return
	}
	defer r.Close()

	err = os.MkdirAll(fmt.Sprintf("/tmp/%s", path.Dir(object)), os.ModePerm)
	if err != nil {
		return
	}

	w, err := os.Create(fmt.Sprintf("/tmp/%s", object))
	if err != nil {
		return
	}
	defer w.Close()

	_, err = io.Copy(w, r)
	return
}

func (d *Data) makeGitCommit(ctx context.Context) error {
	cmd := exec.CommandContext(ctx, "git",
		"add", d.localRelativeFilePath())
	cmd.Dir = d.localRepoPath()
	if out, err := cmd.Output(); err != nil {
		return worker_error.New(err, string(out))
	}

	cmd = exec.CommandContext(ctx, "git",
		"commit", "-m", fmt.Sprintf("%s", d.CommitMessage))
	cmd.Dir = d.localRepoPath()
	if out, err := cmd.Output(); err != nil {
		return worker_error.New(err, string(out))
	}

	return nil
}

func (d *Data) pushToRemote(ctx context.Context) error {
	cmd := exec.CommandContext(ctx, "git",
		"push", "-u", "origin", "HEAD")
	cmd.Dir = d.localRepoPath()
	if out, err := cmd.Output(); err != nil {
		return worker_error.New(err, string(out))
	}

	return nil
}

func (d *Data) cleanup() {
	_ = os.RemoveAll(d.localWorkingDir())
}

func (d *Data) localWorkingDir() string {
	return fmt.Sprintf("/tmp/%s", d.UserID)
}

func (d *Data) localRepoPath() string {
	return fmt.Sprintf("%s/%s", d.localWorkingDir(), d.RepoName)
}

func (d *Data) localProjectPath() string {
	return fmt.Sprintf("%s/%s", d.localRepoPath(), d.ProjectName)
}

func (d *Data) localRelativeFilePath() string {
	return fmt.Sprintf("%s/%s", d.ProjectName, d.FileName)
}

func (d *Data) localFilePath() string {
	return fmt.Sprintf("%s/%s", d.localRepoPath(), d.localRelativeFilePath())
}
