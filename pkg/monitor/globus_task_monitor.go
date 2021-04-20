package monitor

import (
	"context"
	"strings"
	"time"

	"github.com/apex/log"
	globus "github.com/materials-commons/goglobus"
	"gorm.io/gorm"
)

type GlobusTaskMonitor struct {
	client              *globus.Client
	db                  *gorm.DB
	endpointID          string
	finishedGlobusTasks map[string]bool
	lastProcessedTime   time.Time
}

func NewGlobusTaskMonitor(client *globus.Client, db *gorm.DB, endpointID string) *GlobusTaskMonitor {
	return &GlobusTaskMonitor{
		client:              client,
		db:                  db,
		endpointID:          endpointID,
		finishedGlobusTasks: make(map[string]bool),
		// set lastProcessedTime to a date far in the past so that we initially match all requests
		lastProcessedTime: time.Date(2009, time.November, 10, 23, 0, 0, 0, time.UTC),
	}
}

func (m *GlobusTaskMonitor) Start(ctx context.Context) {
	log.Infof("Starting globus task monitor...")
	go m.monitorAndProcessTasks(ctx)
}

func (m *GlobusTaskMonitor) monitorAndProcessTasks(ctx context.Context) {
	for {
		m.retrieveAndProcessUploads(ctx)
		select {
		case <-ctx.Done():
			log.Infof("Shutting down globus monitoring...")
			return
		case <-time.After(10 * time.Second):
		}
	}
}

func (m *GlobusTaskMonitor) retrieveAndProcessUploads(c context.Context) {
	// Build a filter to get all successful tasks that completed in the last week
	lastWeek := time.Now().AddDate(0, 0, -7).Format("2006-01-02")
	taskFilter := map[string]string{
		"filter_completion_time": lastWeek,
		"filter_status":          "SUCCEEDED",
		"orderby":                "completion_time ASC",
		"limit":                  "1000",
	}
	tasks, err := m.client.GetEndpointTaskList(m.endpointID, taskFilter)

	if err != nil {
		log.Infof("globus.GetEndpointTaskList returned the following error: %s - %#v", err, m.client.GetGlobusErrorResponse())
		return
	}

	for _, task := range tasks.Tasks {
		if !m.processTask(task) {
			continue
		}

		//log.Infof("Getting successful transfers for Globus Task %s", task.TaskID)
		transfers, err := m.client.GetTaskSuccessfulTransfers(task.TaskID, 0)

		switch {
		case err != nil:
			log.Infof("globus.GetTaskSuccessfulTransfers(%d) returned error %s - %#v", task.TaskID, err, m.client.GetGlobusErrorResponse())
			continue
		case len(transfers.Transfers) == 0:
			// No files transferred in this request
			continue
		default:
			// Files were transferred for this request
			m.processTransfers(&transfers)
		}

		// Check if we should stop processing requests
		select {
		case <-c.Done():
			break
		default:
		}
	}
}

func (m *GlobusTaskMonitor) processTask(task globus.Task) bool {
	taskCompletionTime, err := time.Parse(time.RFC3339, task.CompletionTime)
	if err != nil {
		log.Errorf("Error parsing task time '%s': %s", task.CompletionTime, err)
		return false
	}

	// task was completed since the last process task, so this task has not yet been processed
	return taskCompletionTime.After(m.lastProcessedTime)
}

func (m *GlobusTaskMonitor) processTransfers(transfers *globus.TransferItems) {
	transferItem := transfers.Transfers[0]

	// Transfer items with a blank DestinationPath are downloads not uploads.
	if transferItem.DestinationPath == "" {
		return
	}

	// Destination path will have the following format: /__transfers/globus/<user-id>/<project-id>/...rest of path...
	// Split will return ["", "__transfers", "globus", "<user-id>", "<project-id>", ...]
	// So the 3rd entry in the array is the id in the globus_uploads table we want to look up.
	pieces := strings.Split(transferItem.DestinationPath, "/")
	if len(pieces) < 5 {
		// sanity check, because the destination path should at least be /__transfers/globus/<user-id>/<project-id>/...rest of path...
		// so it should at least have 5 entries in it (See Split return description above)
		log.Infof("Invalid globus DestinationPath: %s", transferItem.DestinationPath)
		return
	}

	id := pieces[2] // id is the 3rd entry in the path
	if _, ok := m.finishedGlobusTasks[id]; ok {
		// We've seen this globus task before and already processed it
		return
	}

	//globusUpload, err := m.globusUploads.GetGlobusUpload(id)
	//if err != nil {
	//	// If we find a Globus task, but no corresponding entry in our database that means at some
	//	// earlier point in time we processed the task by turning it into a file load request and
	//	// deleting globus upload from our database. So this is an old reference we can just ignore.
	//	// Add the entry to our hash table of completed requests.
	//	m.finishedGlobusTasks[id] = true
	//	return
	//}

	// At this point we have a globus upload. What we are going to do is remove the ACL on the directory
	// so no more files can be uploaded to it. Then we are going to add that directory to the list of
	// directories to upload. Then the file loader will eventually get around to loading these files. In
	// the meantime since we've now created a file load from this globus upload we can delete the entry
	// from the globus_uploads table. Finally we are going to update the status for this background process.

	log.Infof("Processing globus upload %s", id)

	//if _, err := m.client.DeleteEndpointACLRule(m.endpointID, globusUpload.GlobusAclID); err != nil {
	//	log.Infof("Unable to delete ACL: %s", err)
	//}

	//flAdd := model.AddFileLoadModel{
	//	ProjectID:      globusUpload.ProjectID,
	//	Owner:          globusUpload.Owner,
	//	Path:           globusUpload.Path,
	//	GlobusUploadID: globusUpload.ID,
	//}

	//if fl, err := m.fileLoads.AddFileLoad(flAdd); err != nil {
	//	log.Infof("Unable to add file load request: %s", err)
	//	return
	//} else {
	//	log.Infof("Created file load (id: %s) for globus upload %s", fl.ID, id)
	//}

	// Delete the globus upload request as we have now turned it into a file loading request
	// and won't have to process this request again. If the server stops while loading the
	// request or there is some other failure, the file loader will take care of picking up
	// where it left off.
	//m.globusUploads.DeleteGlobusUpload(id)
}
