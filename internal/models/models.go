package models

import "database/sql"

type StageUserDataParams struct {
	Username            string
	UserSlug            string
	UserAvatar          sql.NullString
	CountryCode         sql.NullString
	CountryName         sql.NullString
	RealName            sql.NullString
	Typename            sql.NullString
	TotalProblemsSolved int32
	TotalSubmissions    int32
}
