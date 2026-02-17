package types

import "time"

type WalletType string

const (
	WalletTypePersonal WalletType = "personal"
	WalletTypeGroup    WalletType = "group"
)

type DocumentType string

const (
	DocumentTypePassport      DocumentType = "passport"
	DocumentTypeVisa          DocumentType = "visa"
	DocumentTypeInsurance     DocumentType = "insurance"
	DocumentTypeVaccination   DocumentType = "vaccination"
	DocumentTypeLoyaltyCard   DocumentType = "loyalty_card"
	DocumentTypeFlightBooking DocumentType = "flight_booking"
	DocumentTypeHotelBooking  DocumentType = "hotel_booking"
	DocumentTypeReservation   DocumentType = "reservation"
	DocumentTypeReceipt       DocumentType = "receipt"
	DocumentTypeOther         DocumentType = "other"
)

type WalletDocument struct {
	ID           string                 `json:"id"`
	UserID       string                 `json:"userId"`
	TripID       *string                `json:"tripId,omitempty"`
	WalletType   WalletType             `json:"walletType"`
	DocumentType DocumentType           `json:"documentType"`
	Name         string                 `json:"name"`
	Description  *string                `json:"description,omitempty"`
	FilePath     string                 `json:"-"` // never expose internal storage path
	FileSize     int64                  `json:"fileSize"`
	MimeType     string                 `json:"mimeType"`
	Metadata     map[string]interface{} `json:"metadata"`
	CreatedAt    time.Time              `json:"createdAt"`
	UpdatedAt    time.Time              `json:"updatedAt"`
}

// Request types

type WalletDocumentCreate struct {
	WalletType   WalletType             `json:"walletType" binding:"required,oneof=personal group"`
	TripID       *string                `json:"tripId,omitempty"`
	DocumentType DocumentType           `json:"documentType" binding:"required"`
	Name         string                 `json:"name" binding:"required,max=255"`
	Description  *string                `json:"description,omitempty"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
}

type WalletDocumentUpdate struct {
	Name         *string                `json:"name,omitempty" binding:"omitempty,max=255"`
	Description  *string                `json:"description,omitempty"`
	DocumentType *DocumentType          `json:"documentType,omitempty"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
}

// Response with download URL
type WalletDocumentResponse struct {
	WalletDocument
	DownloadURL string `json:"downloadUrl,omitempty"`
}
