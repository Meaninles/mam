package asset

type LibraryRecord struct {
	ID            string   `json:"id"`
	Name          string   `json:"name"`
	RootLabel     string   `json:"rootLabel"`
	ItemCount     string   `json:"itemCount"`
	Health        string   `json:"health"`
	StoragePolicy string   `json:"storagePolicy"`
	EndpointNames []string `json:"endpointNames"`
}

type CreateLibraryRequest struct {
	Name string `json:"name"`
}

type CreateLibraryResponse struct {
	Message string        `json:"message"`
	Library LibraryRecord `json:"library"`
}

type CreateDirectoryRequest struct {
	ParentID *string `json:"parentId,omitempty"`
	Name     string  `json:"name"`
}

type CreateDirectoryResponse struct {
	Message string      `json:"message"`
	Entry   EntryRecord `json:"entry"`
}

type DeleteEntryResponse struct {
	Message string `json:"message"`
}

type CreateReplicateJobRequest struct {
	EntryIDs     []string `json:"entryIds"`
	EndpointName string   `json:"endpointName"`
}

type CreateDeleteReplicaJobRequest struct {
	EntryIDs     []string `json:"entryIds"`
	EndpointName string   `json:"endpointName"`
}

type CreateDeleteAssetJobRequest struct {
	EntryIDs []string `json:"entryIds"`
}

type Breadcrumb struct {
	ID    *string `json:"id"`
	Label string  `json:"label"`
}

type BrowseQuery struct {
	ParentID                 *string
	Page                     int
	PageSize                 int
	SearchText               string
	FileType                 string
	StatusFilter             string
	SortValue                string
	SortDirection            string
	PartialSyncEndpointNames []string
}

type EntryEndpoint struct {
	MountID      string `json:"mountId"`
	Name         string `json:"name"`
	State        string `json:"state"`
	Tone         string `json:"tone"`
	LastSyncAt   string `json:"lastSyncAt"`
	EndpointType string `json:"endpointType"`
}

type EntryRecord struct {
	ID             string          `json:"id"`
	LibraryID      string          `json:"libraryId"`
	ParentID       *string         `json:"parentId"`
	Type           string          `json:"type"`
	LifecycleState string          `json:"lifecycleState"`
	Name           string          `json:"name"`
	FileKind       string          `json:"fileKind"`
	DisplayType    string          `json:"displayType"`
	ModifiedAt     string          `json:"modifiedAt"`
	CreatedAt      string          `json:"createdAt"`
	Size           string          `json:"size"`
	Path           string          `json:"path"`
	SourceLabel    string          `json:"sourceLabel"`
	LastTaskText   string          `json:"lastTaskText"`
	LastTaskTone   string          `json:"lastTaskTone"`
	Rating         int             `json:"rating"`
	ColorLabel     string          `json:"colorLabel"`
	Badges         []string        `json:"badges"`
	RiskTags       []string        `json:"riskTags"`
	Tags           []string        `json:"tags"`
	Endpoints      []EntryEndpoint `json:"endpoints"`
}

type BrowseLibraryResponse struct {
	Breadcrumbs         []Breadcrumb  `json:"breadcrumbs"`
	Items               []EntryRecord `json:"items"`
	Total               int           `json:"total"`
	CurrentPathChildren int           `json:"currentPathChildren"`
	EndpointNames       []string      `json:"endpointNames"`
}

type ScanDirectoryRequest struct {
	ParentID *string `json:"parentId,omitempty"`
}

type ScanDirectoryResponse struct {
	Message string `json:"message"`
}

type UploadSelectionFile struct {
	Name         string
	RelativePath string
	Size         int64
	Content      []byte
}

type UploadSelectionRequest struct {
	ParentID *string               `json:"parentId,omitempty"`
	Mode     string                `json:"mode"`
	Files    []UploadSelectionFile `json:"files"`
}

type UploadSelectionResponse struct {
	Message      string `json:"message"`
	CreatedCount int    `json:"createdCount"`
	JobID        string `json:"jobId,omitempty"`
}

type UpdateAnnotationsRequest struct {
	Rating     int      `json:"rating"`
	ColorLabel string   `json:"colorLabel"`
	Tags       []string `json:"tags"`
}

type UpdateAnnotationsResponse struct {
	Message string `json:"message"`
}
