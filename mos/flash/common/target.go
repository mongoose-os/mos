package common

import (
	"context"
)

type TargetMemReader interface {
	// ReadTargetReg reads a single 32-bit word from the target (handy for reading registers).
	ReadTargetReg(ctx context.Context, addr uint32) (uint32, error)
	// ReadTargetMem reads length bytes at the specified address in the target's memory.
	// Both addr and length must be word-aligned.
	ReadTargetMem(ctx context.Context, addr uint32, length int) ([]uint32, error)
}

type TargetMemWriter interface {
	// WriteTargetReg writes a single 32-bit word to the target.
	WriteTargetReg(ctx context.Context, addr uint32, value uint32) error
	// WriteTargetMem writes data at the specified address to the target's memory.
	// addr must be word-aligned.
	WriteTargetMem(ctx context.Context, addr uint32, data []uint32) error
}

type TargetMemReaderWriter interface {
	TargetMemReader
	TargetMemWriter
}

type Target interface {
	// ResetRun resets the system and lets it run without debug.
	ResetRun(ctx context.Context) error
	// ResetHalt performs reset and halts the system in debug mode.
	ResetHalt(ctx context.Context) error
	// GetReg retrieves current value of a core register.
	GetReg(ctx context.Context, reg int) (uint32, error)
	// SetReg sets value of a core register.
	SetReg(ctx context.Context, reg int, value uint32) error
	// GetRegs retrieves current values of the core registers.
	GetRegs(ctx context.Context, regFilePtr interface{}) error
	// SetRegs sets values of the core registers.
	SetRegs(ctx context.Context, regFile interface{}) error
	// Run releases the processor from halt and lets it run (from current instruction pointer).
	// If waitHalt is set, will wait for the processor to halt again before returning.
	Run(ctx context.Context, waitHalt bool) error
	// WaitHalt waits for core to halt.
	WaitHalt(ctx context.Context) error
}
