package cosmos

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	sdkmath "cosmossdk.io/math"
	"go.uber.org/zap"
)

const queryPath = "/ethermint.feemarket.v1.Query/BaseFee"

// DynamicFee queries the dynamic gas price base fee and returns a string with the base fee and token denom concatenated.
// If the chain does not have dynamic fees enabled in the config, nothing happens and an empty string is always returned.
func (cc *CosmosProvider) DynamicFee(ctx context.Context) string {
	if !cc.PCfg.DynamicGasPrice {
		return ""
	}

	dynamicFee, err := cc.QueryBaseFee(ctx)
	if err != nil {
		// If there was an error querying the dynamic base fee, do nothing and fall back to configured gas price.
		cc.log.Warn("Failed to query the dynamic gas price base fee", zap.Error(err))
		return ""
	}

	return dynamicFee
}

// QueryBaseFee attempts to make an ABCI query to retrieve the base fee on chains using the Osmosis EIP-1559 implementation.
// This is currently hardcoded to only work on Osmosis.
func (cc *CosmosProvider) QueryBaseFee(ctx context.Context) (string, error) {
	resp, err := cc.ConsensusClient.GetABCIQuery(ctx, queryPath, nil)
	if err != nil || resp.Code != 0 {
		return "", err
	}

	// Clean the response value of any non-numeric characters
	cleanValue := strings.Map(func(r rune) rune {
		if r >= '0' && r <= '9' {
			return r
		}
		return -1
	}, resp.ValueCleaned())

	// Parse the cleaned value as integer
	rawInt, ok := sdkmath.NewIntFromString(cleanValue)
	if !ok {
		return "", fmt.Errorf("failed to parse base fee as integer: %s", cleanValue)
	}

	// Convert to decimal with 18 decimal places
	decFee := sdkmath.LegacyNewDecFromInt(rawInt).Quo(sdkmath.LegacyNewDec(1e18))

	baseFee, err := decFee.Float64()
	if err != nil {
		return "", err
	}

	denom, err := parseTokenDenom(cc.PCfg.GasPrices)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("%f%s", baseFee, denom), nil
}

// parseTokenDenom takes a string in the format numericGasPrice + tokenDenom (e.g. 0.0025uosmo),
// and parses the tokenDenom portion (e.g. uosmo) before returning just the token denom.
func parseTokenDenom(gasPrice string) (string, error) {
	regex := regexp.MustCompile(`^0\.\d+([a-zA-Z]+)$`)

	matches := regex.FindStringSubmatch(gasPrice)

	if len(matches) != 2 {
		return "", fmt.Errorf("failed to parse token denom from string %s", gasPrice)
	}

	return matches[1], nil
}
