package config

import (
	persist "Distributed_Cache/aof"
	"fmt"
	"time"
)

// Example configurations for different use cases

// HighDurabilityConfig provides maximum data safety
func HighDurabilityConfig() *persist.AofConfig {
	return &persist.AofConfig{
		FsyncPolicy:    persist.FsyncAlways, // Sync after every write
		RewriteEnabled: true,
		RewriteSize:    32 * 1024 * 1024, // 32MB
		RewriteMinAge:  30 * time.Minute, // Rewrite after 30 minutes
	}
}

// HighPerformanceConfig prioritizes speed over durability
func HighPerformanceConfig() *persist.AofConfig {
	return &persist.AofConfig{
		FsyncPolicy:    persist.FsyncNo, // Let OS handle syncing
		RewriteEnabled: true,
		RewriteSize:    128 * 1024 * 1024, // 128MB
		RewriteMinAge:  2 * time.Hour,     // Rewrite after 2 hours
	}
}

// BalancedConfig provides a good balance between performance and durability
func BalancedConfig() *persist.AofConfig {
	return &persist.AofConfig{
		FsyncPolicy:    persist.FsyncEverySec, // Sync every second
		RewriteEnabled: true,
		RewriteSize:    64 * 1024 * 1024, // 64MB
		RewriteMinAge:  time.Hour,        // Rewrite after 1 hour
	}
}

// DevelopmentConfig for development/testing
func DevelopmentConfig() *persist.AofConfig {
	return &persist.AofConfig{
		FsyncPolicy:    persist.FsyncEverySec,
		RewriteEnabled: false, // Disable rewrite for simplicity
		RewriteSize:    1024,  // 1KB (very small for testing)
		RewriteMinAge:  time.Minute,
	}
}

// PrintConfigExamples prints all available configuration examples
func PrintConfigExamples() {
	fmt.Println("AOF Configuration Examples")
	fmt.Println("=========================")

	// Example 1: High Durability Setup
	fmt.Println("\n1. High Durability Configuration:")
	config1 := HighDurabilityConfig()
	fmt.Printf("   Fsync Policy: %d (Always)\n", config1.FsyncPolicy)
	fmt.Printf("   Rewrite Enabled: %t\n", config1.RewriteEnabled)
	fmt.Printf("   Rewrite Size: %d bytes\n", config1.RewriteSize)
	fmt.Printf("   Rewrite Min Age: %v\n", config1.RewriteMinAge)

	// Example 2: High Performance Setup
	fmt.Println("\n2. High Performance Configuration:")
	config2 := HighPerformanceConfig()
	fmt.Printf("   Fsync Policy: %d (No)\n", config2.FsyncPolicy)
	fmt.Printf("   Rewrite Enabled: %t\n", config2.RewriteEnabled)
	fmt.Printf("   Rewrite Size: %d bytes\n", config2.RewriteSize)
	fmt.Printf("   Rewrite Min Age: %v\n", config2.RewriteMinAge)

	// Example 3: Balanced Setup
	fmt.Println("\n3. Balanced Configuration:")
	config3 := BalancedConfig()
	fmt.Printf("   Fsync Policy: %d (Every Second)\n", config3.FsyncPolicy)
	fmt.Printf("   Rewrite Enabled: %t\n", config3.RewriteEnabled)
	fmt.Printf("   Rewrite Size: %d bytes\n", config3.RewriteSize)
	fmt.Printf("   Rewrite Min Age: %v\n", config3.RewriteMinAge)

	// Example 4: Development Setup
	fmt.Println("\n4. Development Configuration:")
	config4 := DevelopmentConfig()
	fmt.Printf("   Fsync Policy: %d (Every Second)\n", config4.FsyncPolicy)
	fmt.Printf("   Rewrite Enabled: %t\n", config4.RewriteEnabled)
	fmt.Printf("   Rewrite Size: %d bytes\n", config4.RewriteSize)
	fmt.Printf("   Rewrite Min Age: %v\n", config4.RewriteMinAge)

	fmt.Println("\nUsage in your main.go:")
	fmt.Println("=====================")
	fmt.Println("// For high durability:")
	fmt.Println("config := config.HighDurabilityConfig()")
	fmt.Println("aof, err := persist.NewAofWithConfig(\"database.aof\", config)")
	fmt.Println("")
	fmt.Println("// For high performance:")
	fmt.Println("config := config.HighPerformanceConfig()")
	fmt.Println("aof, err := persist.NewAofWithConfig(\"database.aof\", config)")
	fmt.Println("")
	fmt.Println("// For balanced approach (recommended):")
	fmt.Println("config := config.BalancedConfig()")
	fmt.Println("aof, err := persist.NewAofWithConfig(\"database.aof\", config)")
}
