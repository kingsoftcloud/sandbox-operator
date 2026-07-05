package openapi

import "strings"

type Credential struct {
	AccessKeyID     string
	SecretAccessKey string
	AccountID       string
	Region          string
}

type Env struct {
	Key   string `json:"Key,omitempty"`
	Value string `json:"Value,omitempty"`
}

type Template struct {
	TemplateID                   string               `json:"TemplateId,omitempty"`
	TemplateName                 string               `json:"TemplateName,omitempty"`
	Description                  string               `json:"Description,omitempty"`
	TemplateCategory             string               `json:"TemplateCategory,omitempty"`
	TemplateType                 string               `json:"TemplateType,omitempty"`
	CPU                          int                  `json:"Cpu,omitempty"`
	Memory                       int                  `json:"Memory,omitempty"`
	Envs                         []Env                `json:"Envs,omitempty"`
	KS3MountConfig               *MountConfig         `json:"Ks3MountConfig,omitempty"`
	KPFSMountConfig              *MountConfig         `json:"KpfsMountConfig,omitempty"`
	Command                      string               `json:"Command,omitempty"`
	Ports                        []int                `json:"Ports,omitempty"`
	KecConfig                    *KecConfig           `json:"KecConfig,omitempty"`
	PreheatConfig                *PreheatConfig       `json:"PreheatConfig,omitempty"`
	CanDelete                    bool                 `json:"CanDelete,omitempty"`
	Status                       string               `json:"Status,omitempty"`
	CreatedAt                    string               `json:"CreatedAt,omitempty"`
	UpdatedAt                    string               `json:"UpdatedAt,omitempty"`
	KlogConfig                   *KlogConfig          `json:"KlogConfig,omitempty"`
	NetworkConfig                *NetworkConfig       `json:"NetworkConfig,omitempty"`
	ImageConfig                  *ImageConfig         `json:"ImageConfig,omitempty"`
	SkillConfig                  *SkillConfig         `json:"SkillConfig,omitempty"`
	AccessKey                    string               `json:"AccessKey,omitempty"`
	InstanceQuota                int                  `json:"InstanceQuota,omitempty"`
	RemainingInstanceQuota       int                  `json:"RemainingInstanceQuota,omitempty"`
	RemainingSystemInstanceQuota int                  `json:"RemainingSystemInstanceQuota,omitempty"`
	DiskSizeGB                   int64                `json:"DiskSize,omitempty"`
	CredentialAccessKeyIDMasked  string               `json:"CredentialAccessKeyIDMasked,omitempty"`
	CustomConfiguration          *CustomConfiguration `json:"CustomConfiguration,omitempty"`
}

func (t Template) Identifier() string {
	return t.TemplateID
}

func (t Template) ImageURL() string {
	if t.ImageConfig == nil {
		return ""
	}
	if t.ImageConfig.ImageTag == "" || strings.Contains(t.ImageConfig.ImageURL, ":") {
		return t.ImageConfig.ImageURL
	}
	return t.ImageConfig.ImageURL + ":" + t.ImageConfig.ImageTag
}

func (t Template) ImageSource() string {
	if t.ImageConfig == nil {
		return ""
	}
	return t.ImageConfig.ImageSource
}

func (t Template) TargetPoolSize() int {
	if t.PreheatConfig == nil || !t.PreheatConfig.PreheatEnable {
		return 0
	}
	return t.PreheatConfig.PreheatNumber
}

func (t Template) PreheatedInstanceNumber() int {
	if t.PreheatConfig == nil {
		return 0
	}
	return t.PreheatConfig.PreheatedInstanceNumber
}

func (t Template) DiskSizeMB() int64 {
	if t.KecConfig != nil && t.KecConfig.SystemDiskSizeGB > 0 {
		return t.KecConfig.SystemDiskSizeGB * 1024
	}
	if t.DiskSizeGB > 0 {
		return t.DiskSizeGB * 1024
	}
	return 0
}

type Sandbox struct {
	InstanceID          string               `json:"InstanceId,omitempty"`
	SandboxName         string               `json:"SandboxName,omitempty"`
	Alias               string               `json:"Alias,omitempty"`
	TemplateID          string               `json:"TemplateId,omitempty"`
	TemplateType        string               `json:"TemplateType,omitempty"`
	TemplateCategory    string               `json:"TemplateCategory,omitempty"`
	Status              string               `json:"Status,omitempty"`
	Timeout             int                  `json:"Timeout,omitempty"`
	CreateTime          string               `json:"CreateTime,omitempty"`
	EndTime             string               `json:"EndTime,omitempty"`
	Endpoint            string               `json:"Endpoint,omitempty"`
	AccessURL           *URLs                `json:"AccessUrl,omitempty"`
	ContainerID         string               `json:"ContainerId,omitempty"`
	CustomConfiguration *CustomConfiguration `json:"CustomConfiguration,omitempty"`
	Envs                []Env                `json:"Envs,omitempty"`
	KS3MountConfig      *MountConfig         `json:"Ks3MountConfig,omitempty"`
	KPFSMountConfig     *MountConfig         `json:"KpfsMountConfig,omitempty"`
	EnvdAccessToken     string               `json:"Token,omitempty"`
}

func (s Sandbox) Identifier() string {
	return s.InstanceID
}

func (s Sandbox) Name() string {
	if s.SandboxName != "" {
		return s.SandboxName
	}
	return s.Alias
}

func (s Sandbox) TemplateIdentifier() string {
	return s.TemplateID
}

type NetworkConfig struct {
	PublicNetworkEnable        bool       `json:"PublicNetworkEnable,omitempty"`
	PrivateNetworkEnable       bool       `json:"PrivateNetworkEnable,omitempty"`
	SharedInternetAccessEnable bool       `json:"SharedInternetAccessEnable,omitempty"`
	VPCConfiguration           *VPCConfig `json:"VpcConfiguration,omitempty"`
}

type VPCConfig struct {
	VPCID            string `json:"VpcId,omitempty"`
	SubnetID         string `json:"SubnetId,omitempty"`
	SecurityGroupID  string `json:"SecurityGroupId,omitempty"`
	CIDRBlock        string `json:"-"`
	AvailabilityZone string `json:"-"`
}

type MountConfig struct {
	EnableKS3   bool         `json:"Ks3Enable,omitempty"`
	EnableKPFS  bool         `json:"KpfsEnable,omitempty"`
	MountPoints []MountPoint `json:"Ks3MountPoints,omitempty"`
	KPFSMounts  []MountPoint `json:"KpfsMountPoints,omitempty"`
	ReadOnly    bool         `json:"ReadOnly,omitempty"`
}

func (m *MountConfig) Points() []MountPoint {
	if m == nil {
		return nil
	}
	if len(m.MountPoints) > 0 {
		return m.MountPoints
	}
	return m.KPFSMounts
}

type MountPoint struct {
	BucketName     string `json:"BucketName,omitempty"`
	FileSystemName string `json:"FileSystemName,omitempty"`
	RemotePath     string `json:"RemotePath,omitempty"`
	LocalMountPath string `json:"LocalMountPath,omitempty"`
	ReadOnly       bool   `json:"ReadOnly,omitempty"`
	Token          string `json:"-"`
}

type CustomConfiguration struct {
	ImageURL string `json:"ImageUrl,omitempty"`
	Port     int    `json:"Port,omitempty"`
	Command  string `json:"Command,omitempty"`
}

type URLs struct {
	CdpURL      string `json:"CdpUrl,omitempty"`
	NoVncURL    string `json:"NoVncUrl,omitempty"`
	Code        string `json:"CodeUrl,omitempty"`
	AppURL      string `json:"AppUrl,omitempty"`
	TerminalURL string `json:"TerminalUrl,omitempty"`
	VscodeURL   string `json:"VscodeUrl,omitempty"`
}

type KlogConfig struct {
	Enabled     bool   `json:"KlogEnable,omitempty"`
	ProjectName string `json:"KlogProjectName,omitempty"`
	PoolName    string `json:"KlogPoolName,omitempty"`
}

type SkillConfig struct {
	Enable            bool     `json:"SkillEnable,omitempty"`
	SpaceIDs          []string `json:"SkillSpaceIds,omitempty"`
	EnablePublicSkill bool     `json:"PublicSkillEnable,omitempty"`
}

type DataDisk struct {
	Type               string `json:"Type,omitempty"`
	SizeGB             int64  `json:"Size,omitempty"`
	DeleteWithInstance bool   `json:"DeleteWithInstance,omitempty"`
	Path               string `json:"Path,omitempty"`
}

type ImageConfig struct {
	ImageSource        string `json:"ImageSource,omitempty"`
	ImageEndpoint      string `json:"ImageEndpoint,omitempty"`
	ImageNamespace     string `json:"ImageNamespace,omitempty"`
	ImageName          string `json:"ImageName,omitempty"`
	CredentialUsername string `json:"CredentialUsername,omitempty"`
	CredentialPassword string `json:"CredentialPwd,omitempty"`
	ImageURL           string `json:"ImageUrl,omitempty"`
	ImageTag           string `json:"ImageTag,omitempty"`
	RegistryInstanceID string `json:"RegistryInstanceId,omitempty"`
}

type KecConfig struct {
	Enabled          bool       `json:"KecEnable,omitempty"`
	InstanceType     string     `json:"InstanceType,omitempty"`
	SystemDiskType   string     `json:"SystemDiskType,omitempty"`
	SystemDiskSizeGB int64      `json:"SystemDiskSize,omitempty"`
	DataDisks        []DataDisk `json:"DataDisks,omitempty"`
}

type PreheatConfig struct {
	PreheatEnable           bool `json:"PreheatEnable,omitempty"`
	PreheatNumber           int  `json:"PreheatNumber,omitempty"`
	PreheatedInstanceNumber int  `json:"PreheatedInstanceNumber,omitempty"`
}

type CreateTemplateRequest struct {
	TemplateName     string         `json:"TemplateName,omitempty"`
	Description      string         `json:"Description,omitempty"`
	TemplateCategory string         `json:"TemplateCategory,omitempty"`
	TemplateType     string         `json:"TemplateType,omitempty"`
	ImageConfig      *ImageConfig   `json:"ImageConfig,omitempty"`
	Ports            []int          `json:"Ports,omitempty"`
	Command          string         `json:"Command,omitempty"`
	KecConfig        *KecConfig     `json:"KecConfig,omitempty"`
	CPU              int            `json:"Cpu,omitempty"`
	Memory           int            `json:"Memory,omitempty"`
	Envs             []Env          `json:"Envs,omitempty"`
	SkillConfig      *SkillConfig   `json:"SkillConfig,omitempty"`
	NetworkConfig    *NetworkConfig `json:"NetworkConfig,omitempty"`
	KlogConfig       *KlogConfig    `json:"KlogConfig,omitempty"`
	KPFSMountConfig  *MountConfig   `json:"KpfsMountConfig,omitempty"`
	KS3MountConfig   *MountConfig   `json:"Ks3MountConfig,omitempty"`
	AccessKey        string         `json:"AccessKey,omitempty"`
	SecretAccessKey  string         `json:"SecretAccessKey,omitempty"`
	PreheatConfig    *PreheatConfig `json:"PreheatConfig,omitempty"`
	InstanceQuota    int            `json:"InstanceQuota,omitempty"`
}

type CreateTemplateResponse struct {
	TemplateID      string `json:"TemplateId,omitempty"`
	KlogProjectName string `json:"KlogProjectName,omitempty"`
	KlogPoolName    string `json:"KlogPoolName,omitempty"`
}

func (r CreateTemplateResponse) Identifier() string {
	return r.TemplateID
}

type UpdateTemplateRequest struct {
	TemplateID string `json:"TemplateId,omitempty"`
	CreateTemplateRequest
}

type StartSandboxRequest struct {
	TemplateID      string       `json:"TemplateId"`
	Timeout         int          `json:"Timeout,omitempty"`
	KS3MountConfig  *MountConfig `json:"Ks3MountConfig,omitempty"`
	KPFSMountConfig *MountConfig `json:"KpfsMountConfig,omitempty"`
	AccessKey       string       `json:"AccessKey,omitempty"`
	SecretAccessKey string       `json:"SecretAccessKey,omitempty"`
	Envs            []Env        `json:"Envs,omitempty"`
}

type StartSandboxResponse struct {
	InstanceID string `json:"InstanceId,omitempty"`
	Endpoint   string `json:"Endpoint,omitempty"`
	TemplateID string `json:"TemplateId,omitempty"`
	Token      string `json:"Token,omitempty"`
	Timeout    int    `json:"Timeout,omitempty"`
}

func (r StartSandboxResponse) Identifier() string {
	return r.InstanceID
}

func (r StartSandboxResponse) TemplateIdentifier() string {
	return r.TemplateID
}

type UpdateSandboxRequest struct {
	InstanceID string `json:"InstanceId,omitempty"`
	Timeout    int    `json:"Timeout,omitempty"`
}

type ListTemplatesRequest struct {
	TemplateName string `json:"TemplateName,omitempty"`
	TemplateType string `json:"TemplateType,omitempty"`
	Status       string `json:"Status,omitempty"`
	PageNum      int    `json:"PageNum,omitempty"`
	PageSize     int    `json:"PageSize,omitempty"`
}

type ListSandboxesRequest struct {
	TemplateID   string `json:"TemplateId,omitempty"`
	TemplateName string `json:"TemplateName,omitempty"`
	State        string `json:"State,omitempty"`
	PageNum      int    `json:"PageNum,omitempty"`
	PageSize     int    `json:"PageSize,omitempty"`
}

type TemplateList struct {
	Items    []Template `json:"Templates,omitempty"`
	Total    int        `json:"TotalCount,omitempty"`
	PageNum  int        `json:"PageNum,omitempty"`
	PageSize int        `json:"PageSize,omitempty"`
}

type SandboxList struct {
	Items    []Sandbox `json:"InstanceSet,omitempty"`
	Total    int       `json:"TotalCount,omitempty"`
	PageNum  int       `json:"PageNum,omitempty"`
	PageSize int       `json:"PageSize,omitempty"`
}

type TemplateResponse struct {
	Template
}

func (r TemplateResponse) Value() Template {
	return r.Template
}

type SandboxResponse struct {
	Sandbox
}

func (r SandboxResponse) Value() Sandbox {
	return r.Sandbox
}
