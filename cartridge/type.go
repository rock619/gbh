package cartridge

type Type uint8

const (
	TypeROMOnly Type = 0x00

	TypeMBC1           Type = 0x01
	TypeMBC1RAM        Type = 0x02
	TypeMBC1RAMBattery Type = 0x03

	TypeMBC2        Type = 0x05
	TypeMBC2Battery Type = 0x06

	TypeMMM01           Type = 0x0B
	TypeMMM01RAM        Type = 0x0C
	TypeMMM01RAMBattery Type = 0x0D

	TypeMBC3TimerBattery    Type = 0x0F
	TypeMBC3TimerRAMBattery Type = 0x10
	TypeMBC3                Type = 0x11
	TypeMBC3RAM             Type = 0x12
	TypeMBC3RAMBattery      Type = 0x13

	TypeMBC5                 Type = 0x19
	TypeMBC5RAM              Type = 0x1A
	TypeMBC5RAMBattery       Type = 0x1B
	TypeMBC5Rumble           Type = 0x1C
	TypeMBC5RumbleRAM        Type = 0x1D
	TypeMBC5RumbleRAMBattery Type = 0x1E

	TypeMBC6 Type = 0x20

	TypeMBC7SensorRumbleRAMBattery Type = 0x22

	TypePocketCamera Type = 0xFC
	TypeBandaiTAMA5  Type = 0xFD
	TypeHudsonHuC3   Type = 0xFE
	TypeHudsonHuC1   Type = 0xFF
)
