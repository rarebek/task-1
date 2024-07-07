package main

import (
	"net/http"
	"sort"
	"strconv"
	"time"

	_ "task/docs"

	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

type StartTaskRequest struct {
	Name string `json:"name" binding:"required"`
}

type StopTaskRequest struct {
	ID int `json:"id" binding:"required"`
}

type UpdateUserRequest struct {
	PassportNumber string `json:"passportNumber"`
}

type AddUserRequest struct {
	PassportNumber string `json:"passportNumber" binding:"required"`
}

type User struct {
	ID             int    `json:"id" gorm:"primaryKey"`
	PassportNumber string `json:"passportNumber"`
}

type Task struct {
	ID     int       `json:"id" gorm:"primaryKey"`
	UserID int       `json:"userId"`
	Name   string    `json:"name"`
	Start  time.Time `json:"start" swaggertype:"string" format:"date-time"`
	End    time.Time `json:"end" swaggertype:"string" format:"date-time"`
}

type TaskWithTotalHours struct {
	Task
	TotalHours float64 `json:"totalHours"`
}

var db *gorm.DB

func main() {
	dsn := "host=localhost user=postgres password=nodirbek dbname=postgres port=5432 sslmode=disable TimeZone=Asia/Shanghai"
	var err error
	db, err = gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		panic("failed to connect database")
	}

	db.AutoMigrate(&User{}, &Task{})

	r := gin.Default()
	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	user := r.Group("/user")
	{
		user.GET("/", getUsers)
		user.GET("/:id/tasks", getUserTasks)
		user.POST("/:id/tasks/start", startTask)
		user.POST("/:id/tasks/stop", stopTask)
		user.DELETE("/:id", deleteUser)
		user.PUT("/:id", updateUser)
		user.POST("/", addUser)
	}

	r.Run()
}

// @Summary     Get Users
// @Description Retrieves all users with filtering and pagination
// @ID          get-users
// @Tags        user
// @Accept      json
// @Produce     json
// @Param       passportNumber query string false "Filter by passport number"
// @Param       page query int false "Page number"
// @Param       pageSize query int false "Page size"
// @Success     200 {array} User
// @Failure     500 {object} map[string]string
// @Router      /user [get]
func getUsers(c *gin.Context) {
	var users []User
	query := db

	if passportNumber := c.Query("passportNumber"); passportNumber != "" {
		query = query.Where("passport_number = ?", passportNumber)
	}

	page, err := strconv.Atoi(c.DefaultQuery("page", "1"))
	if err != nil || page < 1 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid page number"})
		return
	}
	pageSize, err := strconv.Atoi(c.DefaultQuery("pageSize", "10"))
	if err != nil || pageSize < 1 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid page size"})
		return
	}
	offset := (page - 1) * pageSize

	query.Offset(offset).Limit(pageSize).Find(&users)
	c.JSON(http.StatusOK, users)
}

// @Summary     Get User Tasks
// @Description Retrieves tasks for a user, calculates total working hours for each task, and returns them sorted by total working hours
// @ID          get-user-tasks
// @Tags        user
// @Accept      json
// @Produce     json
// @Param       id path int true "User ID"
// @Success     200 {array} TaskWithTotalHours
// @Failure     500 {object} map[string]string
// @Router      /user/{id}/tasks [get]
func getUserTasks(c *gin.Context) {
	userID := c.Param("id")
	var tasks []Task
	if err := db.Where("user_id = ?", userID).Find(&tasks).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	var tasksWithTotalHours []TaskWithTotalHours

	for _, task := range tasks {
		duration := task.End.Sub(task.Start)
		totalHours := duration.Hours()

		taskWithTotalHours := TaskWithTotalHours{
			Task:       task,
			TotalHours: totalHours,
		}

		tasksWithTotalHours = append(tasksWithTotalHours, taskWithTotalHours)
	}

	sort.Slice(tasksWithTotalHours, func(i, j int) bool {
		return tasksWithTotalHours[i].TotalHours > tasksWithTotalHours[j].TotalHours
	})

	c.JSON(http.StatusOK, tasksWithTotalHours)
}

// @Summary     Start Task
// @Description Starts a task for a user
// @ID          start-task
// @Tags        user
// @Accept      json
// @Produce     json
// @Param       id path int true "User ID"
// @Param       task body StartTaskRequest true "Task"
// @Success     200 {object} Task
// @Failure     400 {object} map[string]string
// @Failure     500 {object} map[string]string
// @Router      /user/{id}/tasks/start [post]
func startTask(c *gin.Context) {
	var req StartTaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	userID, _ := strconv.Atoi(c.Param("id"))
	task := Task{
		Name:   req.Name,
		UserID: userID,
		Start:  time.Now(),
	}

	if err := db.Create(&task).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, task)
}

// @Summary     Stop Task
// @Description Stops a task for a user
// @ID          stop-task
// @Tags        user
// @Accept      json
// @Produce     json
// @Param       id path int true "User ID"
// @Param       task body StopTaskRequest true "Task"
// @Success     200 {object} Task
// @Failure     400 {object} map[string]string
// @Failure     500 {object} map[string]string
// @Router      /user/{id}/tasks/stop [post]
func stopTask(c *gin.Context) {
	var req StopTaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var task Task
	if err := db.First(&task, req.ID).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Task not found"})
		return
	}

	task.End = time.Now()

	if err := db.Save(&task).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, task)
}

// @Summary     Delete User
// @Description Deletes a user
// @ID          delete-user
// @Tags        user
// @Accept      json
// @Produce     json
// @Param       id path int true "User ID"
// @Success     200 {object} map[string]string
// @Failure     404 {object} map[string]string
// @Failure     500 {object} map[string]string
// @Router      /user/{id} [delete]
func deleteUser(c *gin.Context) {
	userID := c.Param("id")
	if err := db.Delete(&User{}, userID).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "User deleted"})
}

// @Summary     Update User
// @Description Updates a user's information
// @ID          update-user
// @Tags        user
// @Accept      json
// @Produce     json
// @Param       id path int true "User ID"
// @Param       user body UpdateUserRequest true "User"
// @Success     200 {object} User
// @Failure     400 {object} map[string]string
// @Failure     404 {object} map[string]string
// @Failure     500 {object} map[string]string
// @Router      /user/{id} [put]
func updateUser(c *gin.Context) {
	var req UpdateUserRequest
	userID := c.Param("id")

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var user User
	if err := db.First(&user, userID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	user.PassportNumber = req.PassportNumber

	if err := db.Save(&user).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, user)
}

// @Summary     Add User
// @Description Adds a new user
// @ID          add-user
// @Tags        user
// @Accept      json
// @Produce     json
// @Param       user body AddUserRequest true "User"
// @Success     200 {object} User
// @Failure     400 {object} map[string]string
// @Failure     500 {object} map[string]string
// @Router      /user [post]
func addUser(c *gin.Context) {
	var req AddUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	user := User{
		PassportNumber: req.PassportNumber,
	}

	if err := db.Create(&user).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, user)
}
