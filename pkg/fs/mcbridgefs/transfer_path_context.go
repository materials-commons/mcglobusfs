package mcbridgefs

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
)

type TransferPathContext struct {
	TransferType string
	UserID       int
	ProjectID    int
	Path         string
}

func (p *TransferPathContext) ProjectPathContext() string {
	return filepath.Join("/", p.TransferType, fmt.Sprintf("%d/%d", p.UserID, p.ProjectID))
}

func (p *TransferPathContext) IsTransferType() bool {
	return p.TransferType != ""
}

func (p *TransferPathContext) IsRoot() bool {
	return p.TransferType == ""
}

func (p *TransferPathContext) IsUserID() bool {
	return p.UserID != 0
}

func (p *TransferPathContext) IsProject() bool {
	return p.ProjectID != 0
}

func (n *Node) ToTransferPathContext() *TransferPathContext {
	basePath := n.Path(n.Root())
	return ToTransferPathContext(filepath.Join("/", basePath))
}

func (p *TransferPathContext) ToFilePath(name string) string {
	return filepath.Join(p.Path, name)
}

func (p *TransferPathContext) ToFSPath(name string) string {
	return filepath.Join("/", p.TransferType, fmt.Sprintf("%d/%d", p.UserID, p.ProjectID), p.Path, name)
}

func ToTransferPathContext(p string) *TransferPathContext {
	pathParts := strings.SplitN(p, "/", 5)

	userID := 0
	if len(pathParts) > 2 {
		userID, _ = strconv.Atoi(pathParts[2])
	}

	projectID := 0
	if len(pathParts) > 3 {
		projectID, _ = strconv.Atoi(pathParts[3])
	}

	rest := ""
	if userID != 0 && projectID != 0 {
		rest = "/"
	}
	if len(pathParts) == 5 {
		rest = filepath.Join("/", pathParts[4])
	}

	return &TransferPathContext{
		TransferType: pathParts[1],
		UserID:       userID,
		ProjectID:    projectID,
		Path:         rest,
	}
}
