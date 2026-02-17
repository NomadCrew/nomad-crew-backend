package types

// ============================================================
// Phase C: LLM Document Extraction Contracts (future sprint)
// API: POST /v1/wallet/documents/:id/extract
// No implementation yet — contracts only.
//
// Architecture:
//   Mobile → On-device OCR (free) →
//   POST /v1/wallet/documents/:id/extract →
//   Google Vision API (fallback if low confidence) →
//   GPT-4o mini (structured extraction) →
//   Return JSON → Pre-fill wallet document metadata
//
// Budget: $5-10/month
//   Google Vision free tier: 1000 requests/month
//   GPT-4o mini: ~$0.01/extraction
//
// Rate limit: 10 extractions/hour per user
// ============================================================

// ExtractionRequest is the request body for document extraction.
type ExtractionRequest struct {
	OCRText      string       `json:"ocrText" binding:"required"`
	DocumentType DocumentType `json:"documentType,omitempty"` // optional hint
}

// ExtractionResponse is returned by the extraction endpoint.
type ExtractionResponse struct {
	ExtractedData interface{}  `json:"extractedData"`
	Confidence    float64      `json:"confidence"` // 0-1 overall
	SuggestedType DocumentType `json:"suggestedType"`
}

// Confidence thresholds:
//   < 0.5: warn user, suggest manual entry
//   0.5-0.8: show editable review screen with pre-filled fields
//   >= 0.8: auto-fill metadata
const (
	ExtractionConfidenceLow  = 0.5
	ExtractionConfidenceHigh = 0.8
)

// FlightBookingExtraction holds extracted fields from a flight booking.
type FlightBookingExtraction struct {
	Type             string  `json:"type"` // "flight_booking"
	PassengerName    string  `json:"passengerName,omitempty"`
	FlightNumber     string  `json:"flightNumber,omitempty"`
	DepartureAirport string  `json:"departureAirport,omitempty"` // IATA code
	ArrivalAirport   string  `json:"arrivalAirport,omitempty"`   // IATA code
	DepartureDate    string  `json:"departureDate,omitempty"`    // ISO date
	ArrivalDate      string  `json:"arrivalDate,omitempty"`      // ISO date
	BookingReference string  `json:"bookingReference,omitempty"`
	Confidence       float64 `json:"confidence"`
}

// HotelBookingExtraction holds extracted fields from a hotel booking.
type HotelBookingExtraction struct {
	Type               string  `json:"type"` // "hotel_booking"
	HotelName          string  `json:"hotelName,omitempty"`
	Address            string  `json:"address,omitempty"`
	CheckInDate        string  `json:"checkInDate,omitempty"`  // ISO date
	CheckOutDate       string  `json:"checkOutDate,omitempty"` // ISO date
	ConfirmationNumber string  `json:"confirmationNumber,omitempty"`
	GuestName          string  `json:"guestName,omitempty"`
	Confidence         float64 `json:"confidence"`
}

// ReceiptExtraction holds extracted fields from a receipt.
type ReceiptExtraction struct {
	Type         string              `json:"type"` // "receipt"
	MerchantName string              `json:"merchantName,omitempty"`
	Date         string              `json:"date,omitempty"` // ISO date
	TotalAmount  *ReceiptAmount      `json:"totalAmount,omitempty"`
	LineItems    []ReceiptLineItem   `json:"lineItems,omitempty"`
	Confidence   float64             `json:"confidence"`
}

// ReceiptAmount represents a monetary amount with currency.
type ReceiptAmount struct {
	Amount   float64 `json:"amount"`
	Currency string  `json:"currency"` // ISO 4217
}

// ReceiptLineItem represents a single line item on a receipt.
type ReceiptLineItem struct {
	Description string  `json:"description"`
	Amount      float64 `json:"amount"`
}

// GenericExtraction is the fallback for unrecognized document types.
type GenericExtraction struct {
	Type       string            `json:"type"` // "other"
	Fields     map[string]string `json:"fields"`
	Confidence float64           `json:"confidence"`
}
