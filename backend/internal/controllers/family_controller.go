package controllers

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gottatouchsomegrass/smart-door-backend/internal/models"
	"github.com/gottatouchsomegrass/smart-door-backend/internal/repository"
	"github.com/gottatouchsomegrass/smart-door-backend/internal/services"
	"github.com/gottatouchsomegrass/smart-door-backend/internal/storage"
)

const maxPhotoSize = 10 << 20 // 10 MB

type FamilyController struct {
	repo        *repository.FamilyRepo
	faceService *services.FaceService
	store       *storage.MediaStorage
}

func NewFamilyController(
	repo *repository.FamilyRepo,
	faceService *services.FaceService,
	store *storage.MediaStorage,
) *FamilyController {
	return &FamilyController{repo: repo, faceService: faceService, store: store}
}

// ListMembers godoc
// @Summary List family members
// @Tags family
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /family [get]
func (fc *FamilyController) ListMembers(c *gin.Context) {
	members, err := fc.repo.List()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list members"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"members": members})
}

// GetMember godoc
// @Summary Get a family member by ID
// @Tags family
// @Produce json
// @Param id path int true "Member ID"
// @Success 200 {object} map[string]interface{}
// @Failure 404 {object} map[string]string
// @Router /family/{id} [get]
func (fc *FamilyController) GetMember(c *gin.Context) {
	id, err := parseID(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	m, err := fc.repo.Get(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "database error"})
		return
	}
	if m == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "member not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"member": m})
}

// CreateMember godoc
// @Summary Create a family member (name only; enroll face separately)
// @Tags family
// @Accept json
// @Produce json
// @Param body body object true "name"
// @Success 201 {object} map[string]interface{}
// @Router /family [post]
func (fc *FamilyController) CreateMember(c *gin.Context) {
	var req struct {
		Name string `json:"name" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "name is required"})
		return
	}

	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "name must not be blank"})
		return
	}

	// Check duplicate
	existing, _ := fc.repo.GetByName(req.Name)
	if existing != nil {
		c.JSON(http.StatusConflict, gin.H{"error": fmt.Sprintf("member %q already exists", req.Name)})
		return
	}

	m := &models.FamilyMember{Name: req.Name}
	if err := fc.repo.Create(m); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create member"})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"member": m})
}

// UpdateMember godoc
// @Summary Rename a family member
// @Tags family
// @Accept json
// @Produce json
// @Param id path int true "Member ID"
// @Router /family/{id} [put]
func (fc *FamilyController) UpdateMember(c *gin.Context) {
	id, err := parseID(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	var req struct {
		Name string `json:"name" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "name is required"})
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "name must not be blank"})
		return
	}

	m, err := fc.repo.Get(id)
	if err != nil || m == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "member not found"})
		return
	}

	// Check new name not taken by someone else
	other, _ := fc.repo.GetByName(req.Name)
	if other != nil && other.ID != m.ID {
		c.JSON(http.StatusConflict, gin.H{"error": fmt.Sprintf("name %q is already taken", req.Name)})
		return
	}

	oldName := m.Name
	m.Name = req.Name

	// If face was enrolled under the old name, re-enroll under the new name
	if m.FaceEnrolled && oldName != req.Name {
		// We can't re-enroll without the original image; unenroll the old name so
		// the user re-enrolls with the new name.
		_ = fc.faceService.DeleteFace(oldName)
		m.FaceEnrolled = false
	}

	if err := fc.repo.Update(m); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update member"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"member": m})
}

// DeleteMember godoc
// @Summary Delete a family member and remove their face enrollment
// @Tags family
// @Param id path int true "Member ID"
// @Success 200 {object} map[string]string
// @Router /family/{id} [delete]
func (fc *FamilyController) DeleteMember(c *gin.Context) {
	id, err := parseID(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	m, err := fc.repo.Get(id)
	if err != nil || m == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "member not found"})
		return
	}

	// Remove face enrollment (best-effort)
	if m.FaceEnrolled {
		if ferr := fc.faceService.DeleteFace(m.Name); ferr != nil {
			// Log but don't fail the delete
			_ = ferr
		}
	}

	if err := fc.repo.Delete(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete member"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"deleted": id})
}

// EnrollFace godoc
// @Summary Upload a photo and enroll the member's face in the recognition model
// @Tags family
// @Accept multipart/form-data
// @Param id path int true "Member ID"
// @Param photo formData file true "Face photo"
// @Success 200 {object} map[string]interface{}
// @Router /family/{id}/enroll [post]
func (fc *FamilyController) EnrollFace(c *gin.Context) {
	id, err := parseID(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	m, err := fc.repo.Get(id)
	if err != nil || m == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "member not found"})
		return
	}

	// Parse multipart photo
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxPhotoSize)
	file, header, err := c.Request.FormFile("photo")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "photo field is required (multipart/form-data)"})
		return
	}
	defer file.Close()

	// Validate content type
	contentType := header.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "image/jpeg"
	}
	if !strings.HasPrefix(contentType, "image/") {
		c.JSON(http.StatusBadRequest, gin.H{"error": "uploaded file must be an image"})
		return
	}

	imageBytes, err := io.ReadAll(file)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to read uploaded file"})
		return
	}
	if len(imageBytes) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "uploaded file is empty"})
		return
	}

	// Upload photo to MinIO first
	objectName := fmt.Sprintf("family/%d_%d.jpg", id, time.Now().UnixMilli())
	photoURL, err := fc.store.UploadImage(context.Background(), objectName, imageBytes, "image/jpeg")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to store photo"})
		return
	}

	// Enroll face in the recognition model
	if err := fc.faceService.EnrollFace(m.Name, imageBytes); err != nil {
		// Enrollment failed — clean up the uploaded photo so storage stays consistent
		_ = fc.store.DeleteObject(context.Background(), objectName)
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}

	// Remove old photo from MinIO if there was one
	if m.PhotoURL != "" && m.FaceEnrolled {
		oldObject := extractObjectName(m.PhotoURL)
		if oldObject != "" {
			_ = fc.store.DeleteObject(context.Background(), oldObject)
		}
	}

	m.PhotoURL = photoURL
	m.FaceEnrolled = true
	if err := fc.repo.Update(m); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "face enrolled but failed to update database"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"member": m})
}

// UnenrollFace godoc
// @Summary Remove the face enrollment for a member
// @Tags family
// @Param id path int true "Member ID"
// @Success 200 {object} map[string]interface{}
// @Router /family/{id}/enroll [delete]
func (fc *FamilyController) UnenrollFace(c *gin.Context) {
	id, err := parseID(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	m, err := fc.repo.Get(id)
	if err != nil || m == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "member not found"})
		return
	}

	if !m.FaceEnrolled {
		c.JSON(http.StatusOK, gin.H{"member": m, "message": "face was not enrolled"})
		return
	}

	if err := fc.faceService.DeleteFace(m.Name); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to remove face from model"})
		return
	}

	m.FaceEnrolled = false
	if err := fc.repo.Update(m); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "face unenrolled but failed to update database"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"member": m})
}

// extractObjectName strips the backend proxy prefix from a stored photo URL
// e.g. "http://host:8080/images/family/1_123.jpg" → "family/1_123.jpg"
func extractObjectName(photoURL string) string {
	const prefix = "/images/"
	idx := strings.Index(photoURL, prefix)
	if idx < 0 {
		return ""
	}
	return photoURL[idx+len(prefix):]
}

func parseID(c *gin.Context) (uint, error) {
	idStr := c.Param("id")
	n, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		return 0, err
	}
	return uint(n), nil
}
