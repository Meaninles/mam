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

type Breadcrumb struct {
	ID    *string `json:"id"`
	Label string  `json:"label"`
}

type BrowseQuery struct {
	ParentID                *string
	Page                    int
	PageSize                int
	SearchText              string
	FileType                string
	StatusFilter            string
	SortValue               string
	SortDirection           string
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

type MetadataRow struct {
	Label string `json:"label"`
	Value string `json:"value"`
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
	Notes          string          `json:"notes"`
	LastTaskText   string          `json:"lastTaskText"`
	LastTaskTone   string          `json:"lastTaskTone"`
	Rating         int             `json:"rating"`
	ColorLabel     string          `json:"colorLabel"`
	Badges         []string        `json:"badges"`
	RiskTags       []string        `json:"riskTags"`
	Tags           []string        `json:"tags"`
	Endpoints      []EntryEndpoint `json:"endpoints"`
	Metadata       []MetadataRow   `json:"metadata"`
}

type BrowseLibraryResponse struct {
	Breadcrumbs         []Breadcrumb  `json:"breadcrumbs"`
	Items               []EntryRecord `json:"items"`
	Total               int           `json:"total"`
	CurrentPathChildren int           `json:"currentPathChildren"`
	EndpointNames       []string      `json:"endpointNames"`
}
