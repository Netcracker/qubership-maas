package api_version

type ApiVersionInfo struct {
	MajorVersion    int                `json:"major"`
	MinorVersion    int                `json:"minor"`
	SupportedMajors []int              `json:"supportedMajors"`
	Specs           []ApiVersionInfoV2 `json:"specs"`
}

type ApiVersionInfoV2 struct {
	Root            string `json:"specRootUrl"`
	MajorVersion    int    `json:"major"`
	MinorVersion    int    `json:"minor"`
	SupportedMajors []int  `json:"supportedMajors"`
}

var ApiVersion = ApiVersionInfo{
	// start backward compatibility part
	MajorVersion:    2,
	MinorVersion:    18,
	SupportedMajors: []int{1, 2},
	// end backward compatibility part

	Specs: []ApiVersionInfoV2{
		{
			Root:            "/api",
			MajorVersion:    2,
			MinorVersion:    18,
			SupportedMajors: []int{1, 2},
		},
		{
			Root:            "/api/bluegreen",
			MajorVersion:    1,
			MinorVersion:    3,
			SupportedMajors: []int{1},
		},
		{
			Root:            "/api/composite",
			MajorVersion:    1,
			MinorVersion:    0,
			SupportedMajors: []int{1},
		},
		{
			Root:            "/api/declarations",
			MajorVersion:    1,
			MinorVersion:    0,
			SupportedMajors: []int{1},
		},
	},
}
