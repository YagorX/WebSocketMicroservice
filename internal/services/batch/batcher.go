package batcher

// import (
// 	"errors"
// 	"time"

// 	models "MicroserviceWebsocket/internal/domain"
// )

// type Service struct {
// 	batchQueue   chan models.BatchItem
// 	maxBatchSize int
// 	batchTimeout time.Duration
// }

// func (s *Service) ProcessRequest(req models.Request) (*models.Response, error) {
// 	// Добавление в батч
// 	future := s.addToBatch(req)

// 	// Ожидание результата
// 	select {
// 	case result := <-future:
// 		return result, nil
// 	case <-time.After(30 * time.Second):
// 		return nil, errors.New("timeout")
// 	}
// }

// func (s *Service) addToBatch(req models.Request) chan *models.Response {
// 	future := make(chan *models.Response, 1)

// 	batchItem := models.BatchItem{
// 		Request: req,
// 		Future:  future,
// 	}

// 	s.batchQueue <- batchItem
// 	return future
// }

// // Worker для обработки батчей
// func (s *Service) StartBatchWorker() {
// 	// for {
// 	// 	batch := s.collectBatch()
// 	// 	go s.processBatch(batch)
// 	// }
// }
