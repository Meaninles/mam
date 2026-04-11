package tag

type LibraryRecord struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type ManagementOverview struct {
	TotalTags            int `json:"totalTags"`
	UsedTagCount         int `json:"usedTagCount"`
	UngroupedTagCount    int `json:"ungroupedTagCount"`
	CrossLibraryTagCount int `json:"crossLibraryTagCount"`
}

type GroupRecord struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	OrderIndex   int    `json:"orderIndex"`
	TagCount     int    `json:"tagCount"`
	UsedTagCount int    `json:"usedTagCount"`
}

type Record struct {
	ID                   string   `json:"id"`
	Name                 string   `json:"name"`
	NormalizedName       string   `json:"normalizedName"`
	GroupID              string   `json:"groupId"`
	GroupName            string   `json:"groupName"`
	OrderIndex           int      `json:"orderIndex"`
	IsPinned             bool     `json:"isPinned"`
	UsageCount           int      `json:"usageCount"`
	LibraryIDs           []string `json:"libraryIds"`
	LinkedLibraryIDs     []string `json:"linkedLibraryIds"`
	OutOfScopeUsageCount int      `json:"outOfScopeUsageCount"`
	CreatedAt            string   `json:"createdAt"`
	UpdatedAt            string   `json:"updatedAt"`
}

type ManagementSnapshot struct {
	Overview  ManagementOverview `json:"overview"`
	Groups    []GroupRecord      `json:"groups"`
	Tags      []Record           `json:"tags"`
	Libraries []LibraryRecord    `json:"libraries"`
}

type Suggestion struct {
	ID         string   `json:"id"`
	Name       string   `json:"name"`
	Count      int      `json:"count"`
	GroupName  string   `json:"groupName"`
	IsPinned   bool     `json:"isPinned"`
	LibraryIDs []string `json:"libraryIds"`
}

type CreateGroupRequest struct {
	Name string `json:"name"`
}

type CreateGroupResponse struct {
	Message string `json:"message"`
	GroupID string `json:"groupId"`
}

type UpdateGroupRequest struct {
	Name string `json:"name"`
}

type MoveRequest struct {
	Direction string `json:"direction"`
}

type CreateTagRequest struct {
	Name      string   `json:"name"`
	GroupID   string   `json:"groupId"`
	LibraryIDs []string `json:"libraryIds"`
	IsPinned  bool     `json:"isPinned"`
}

type CreateTagResponse struct {
	Message string `json:"message"`
	TagID   string `json:"tagId"`
}

type UpdateTagRequest struct {
	Name      string   `json:"name"`
	GroupID   string   `json:"groupId"`
	LibraryIDs []string `json:"libraryIds"`
	IsPinned  bool     `json:"isPinned"`
}

type MergeTagRequest struct {
	TargetID string `json:"targetId"`
}

type MutationResponse struct {
	Message string `json:"message"`
}
