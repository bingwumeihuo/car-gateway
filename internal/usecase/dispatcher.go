package usecase

import (
	"context"
	"sync"

	"go.uber.org/zap"
)

type DataDispatcher struct {
	dataChan    chan interface{}
	producer    DataProducer
	logger      *zap.Logger
	workerCount int
	ctx         context.Context
	cancel      context.CancelFunc
	wg          sync.WaitGroup
}

// NewDataDispatcher 创建一个新的数据分发器
func NewDataDispatcher(producer DataProducer, workerCount int, logger *zap.Logger) *DataDispatcher {
	ctx, cancel := context.WithCancel(context.Background())
	return &DataDispatcher{
		dataChan:    make(chan interface{}, 10000), // 带缓冲 Channel，防止阻塞
		producer:    producer,
		workerCount: workerCount,
		logger:      logger,
		ctx:         ctx,
		cancel:      cancel,
	}
}

// Start 启动 worker 协程池
func (d *DataDispatcher) Start() {
	for i := 0; i < d.workerCount; i++ {
		d.wg.Add(1)
		go d.worker(i)
	}
	d.logger.Info("DataDispatcher started", zap.Int("workers", d.workerCount))
}

// Stop 停止分发器并等待所有 worker 退出
func (d *DataDispatcher) Stop() {
	d.cancel() // 通知 worker 退出
	d.wg.Wait()
	close(d.dataChan)
	d.logger.Info("DataDispatcher stopped")
}

// Dispatch 将数据投递到缓冲通道 (非阻塞，如果满则丢弃或记录)
func (d *DataDispatcher) Dispatch(data interface{}) {
	select {
	case d.dataChan <- data:
		// 成功投递
	default:
		// 通道已满，丢弃数据或记录错误 metrics
		d.logger.Warn("DataDispatcher channel full, dropping data")
	}
}

func (d *DataDispatcher) worker(id int) {
	defer d.wg.Done()
	for {
		select {
		case <-d.ctx.Done():
			return
		case data := <-d.dataChan:
			d.process(data)
		}
	}
}

func (d *DataDispatcher) process(data interface{}) {
	if err := d.producer.Produce(d.ctx, "vehicle_data", "", data); err != nil {
		d.logger.Error("DataDispatcher failed to send data", zap.Error(err))
	}

}
