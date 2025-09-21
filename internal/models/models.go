package models

type StageUserDataParams struct {
	Username            string
	UserSlug            string
	UserAvatar          string
	CountryCode         string
	CountryName         string
	RealName            string
	Typename            string
	TotalProblemsSolved int32
	TotalSubmissions    int32
}
