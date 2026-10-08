package authcontroller

type registerRequest struct {
	Name     string `json:"name" binding:"required" minLength:"2" maxLength:"50"`
	Email    string `json:"email" binding:"required" maxLength:"254"`
	Password string `json:"password" binding:"required" minLength:"15" maxLength:"128"`
}

type loginRequest struct {
	Email    string `json:"email" binding:"required" maxLength:"254"`
	Password string `json:"password" binding:"required" minLength:"15" maxLength:"128"`
}

type profileRequest struct {
	Name         string `json:"name" binding:"required" minLength:"2" maxLength:"50"`
	RemoveAvatar bool   `json:"remove_avatar"`
}
