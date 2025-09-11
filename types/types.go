package types


type LoginInput struct {
	Email string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

type SignupInput struct {
	UserName string `json:"username" binding:"required"`
	Email string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=6"`
}