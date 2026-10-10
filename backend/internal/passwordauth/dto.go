package passwordauth

type signUpDto struct {
	Username string `json:"username" binding:"required,username,min=2,max=50" unorm:"nfc"`
	Email    string `json:"email" binding:"required,email,max=254" unorm:"nfc"`
	Password string `json:"password" binding:"required,max=256"`
}

type tokenDto struct {
	Token string `json:"token" binding:"required,max=100"`
}

type emailDto struct {
	Email string `json:"email" binding:"required,max=254" unorm:"nfc"`
}

type loginDto struct {
	Identifier string `json:"identifier" binding:"required,max=254" unorm:"nfc"`
	Password   string `json:"password" binding:"required,max=256"`
}

type resetDto struct {
	Token    string `json:"token" binding:"required,max=100"`
	Password string `json:"password" binding:"required,max=256"`
}

type changePasswordDto struct {
	CurrentPassword string `json:"currentPassword" binding:"max=256"`
	NewPassword     string `json:"newPassword" binding:"required,max=256"`
}

type importHashDto struct {
	PasswordHash string `json:"passwordHash" binding:"required,max=1024"`
}

type passwordStatusDto struct {
	HasPassword bool   `json:"hasPassword"`
	Algorithm   string `json:"algorithm,omitempty"`
}
