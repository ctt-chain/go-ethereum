package dpos

import (
	"encoding/binary"
	"errors"
	"fmt"
	"math/big"
	"math/rand"
	"sort"

	"github.com/ctt-chain/go-ethereum/common"
	"github.com/ctt-chain/go-ethereum/core/state"
	"github.com/ctt-chain/go-ethereum/core/types"
	"github.com/ctt-chain/go-ethereum/crypto"
	"github.com/ctt-chain/go-ethereum/log"
	"github.com/ctt-chain/go-ethereum/trie"
	"encoding/json"
)

type EpochContext struct {
	TimeStamp   int64
	DposContext *types.DposContext
	statedb     *state.StateDB
}

// countVotes
func (ec *EpochContext) countVotes() (votes map[common.Address]*big.Int, err error) {
	votes = map[common.Address]*big.Int{}
	delegateTrie := ec.DposContext.DelegateTrie()
	candidateTrie := ec.DposContext.CandidateTrie()
	//statedb := ec.statedb

	iterCandidate := trie.NewIterator(candidateTrie.NodeIterator(nil))
	existCandidate := iterCandidate.Next()
	if !existCandidate {
		return votes, errors.New("no candidates")
	}
	for existCandidate {
		candidate := iterCandidate.Value
		candidateAddr := common.BytesToAddress(candidate)
		delegateIterator := trie.NewIterator(delegateTrie.PrefixIterator(candidate))
		existDelegator := delegateIterator.Next()
		if !existDelegator {
			votes[candidateAddr] = new(big.Int)
			existCandidate = iterCandidate.Next()
			continue
		}
		for existDelegator {
			delegator := delegateIterator.Value
			score, ok := votes[candidateAddr]
			if !ok {
				score = new(big.Int)
			}
			//delegatorAddr := common.BytesToAddress(delegator)
			//weight := statedb.GetBalance(delegatorAddr)
			//balance replace
			level := ec.statedb.GetMinerLevel(common.BytesToAddress(delegator))
			score.Add(score, new(big.Int).SetUint64(level))
			votes[candidateAddr] = score
			existDelegator = delegateIterator.Next()
		}
		existCandidate = iterCandidate.Next()
	}
	return votes, nil
}

func (ec *EpochContext) kickoutValidator(epoch int64) error {
	validators, err := ec.DposContext.GetValidators()
	if err != nil {
		return fmt.Errorf("failed to get validator: %s", err)
	}
	if len(validators) == 0 {
		return errors.New("no validator could be kickout")
	}

	epochDuration := epochInterval
	// First epoch duration may lt epoch interval,
	// while the first block time wouldn't always align with epoch interval,
	// so caculate the first epoch duartion with first block time instead of epoch interval,
	// prevent the validators were kickout incorrectly.
	if ec.TimeStamp-timeOfFirstBlock < epochInterval {
		epochDuration = ec.TimeStamp - timeOfFirstBlock
	}

	needKickoutValidators := sortableAddresses{}
	for _, validator := range validators {
		key := make([]byte, 8)
		binary.BigEndian.PutUint64(key, uint64(epoch))
		key = append(key, validator.Bytes()...)
		cnt := int64(0)
		if cntBytes := ec.DposContext.MintCntTrie().Get(key); cntBytes != nil {
			cnt = int64(binary.BigEndian.Uint64(cntBytes))
		}
		if cnt < epochDuration/blockInterval/maxValidatorSize/2 {
			// not active validators need kickout
			needKickoutValidators = append(needKickoutValidators, &sortableAddress{validator, big.NewInt(cnt)})
		}
	}
	// no validators need kickout
	needKickoutValidatorCnt := len(needKickoutValidators)
	if needKickoutValidatorCnt <= 0 {
		return nil
	}
	sort.Sort(sort.Reverse(needKickoutValidators))

	candidateCount := 0
	iter := trie.NewIterator(ec.DposContext.CandidateTrie().NodeIterator(nil))
	for iter.Next() {
		candidateCount++
		if candidateCount >= needKickoutValidatorCnt+safeSize {
			break
		}
	}

	for i, validator := range needKickoutValidators {
		// ensure candidate count greater than or equal to safeSize
		if candidateCount <= safeSize {
			log.Info("No more candidate can be kickout", "prevEpochID", epoch, "candidateCount", candidateCount, "needKickoutCount", len(needKickoutValidators)-i)
			return nil
		}

		if err := ec.DposContext.KickoutCandidate(validator.address); err != nil {
			return err
		}
		// if kickout success, candidateCount minus 1
		candidateCount--
		log.Info("Kickout candidate", "prevEpochID", epoch, "candidate", validator.address.String(), "mintCnt", validator.weight.String())
	}
	return nil
}

func (ec *EpochContext) lookupValidator(now int64) (validator common.Address, err error) {
	validator = common.Address{}
	offset := now % epochInterval
	if offset%blockInterval != 0 {
		return common.Address{}, ErrInvalidMintBlockTime
	}
	offset /= blockInterval

	validators, err := ec.DposContext.GetValidators()
	if err != nil {
		return common.Address{}, err
	}
	validatorSize := len(validators)
	if validatorSize == 0 {
		return common.Address{}, errors.New("failed to lookup validator")
	}
	offset %= int64(validatorSize)
	return validators[offset], nil
}

func (ec *EpochContext) GetDelegate() (*trie.Trie) {
	return ec.DposContext.GetDelegate()
}

func (ec *EpochContext) GetVoteTrie() (*trie.Trie) {
	return ec.DposContext.GetVoteTrie()
}

//返回传入数字的阶和   7         12
func getDecAdd(num int64, perCand int64) (int64) {
	var sum int64 = 0
	for true {

		for i := num; i > 0; i-- {
			perCand = perCand - 1

			sum = sum + i
			if perCand <= 0 {
				break
			}
			//fmt.Println(sum)
		}
		if perCand <= 0 {
			break
		}
	}
	fmt.Println(sum)
	return sum
}
func (ec *EpochContext) tryElect(genesis, parent *types.Header) error {
	genesisEpoch := genesis.Time.Int64() / epochInterval
	prevEpoch := parent.Time.Int64() / epochInterval
	currentEpoch := ec.TimeStamp / epochInterval

	prevEpochIsGenesis := prevEpoch == genesisEpoch
	if prevEpochIsGenesis && prevEpoch < currentEpoch {
		prevEpoch = currentEpoch - 1
	}

	prevEpochBytes := make([]byte, 8)
	binary.BigEndian.PutUint64(prevEpochBytes, uint64(prevEpoch))
	iter := trie.NewIterator(ec.DposContext.MintCntTrie().PrefixIterator(prevEpochBytes))
	for i := prevEpoch; i < currentEpoch; i++ {
		// if prevEpoch is not genesis, kickout not active candidate
		if !prevEpochIsGenesis && iter.Next() {
			if err := ec.kickoutValidator(prevEpoch); err != nil {
				return err
			}
		}
		votes, err := ec.countVotes()
		currentVoteMap = votes
		if err != nil {
			return err
		}
		candidates := sortableAddresses{}
		for candidate, cnt := range votes {
			candidates = append(candidates, &sortableAddress{candidate, cnt})
		}
		if len(candidates) < safeSize {
			return errors.New("too few candidates")
		}
		sort.Sort(candidates)
		if len(candidates) > maxValidatorSize {
			candidates = candidates[:maxValidatorSize]
		}

		// shuffle candidates
		seed := int64(binary.LittleEndian.Uint32(crypto.Keccak512(parent.Hash().Bytes()))) + i
		r := rand.New(rand.NewSource(seed))
		for i := len(candidates) - 1; i > 0; i-- {
			j := int(r.Int31n(int32(i + 1)))
			candidates[i], candidates[j] = candidates[j], candidates[i]
		}
		sortedValidators := make([]common.Address, 0)
		for _, candidate := range candidates {
			sortedValidators = append(sortedValidators, candidate.address)
		}

		//in there , Start to calculate the super node reward

		//get curr epoch reword
		//ret := make(map[string]big.Int)
		var ret = make(map[common.Address]*big.Int)
		for k := range stages {
			number, flag := new(big.Int).SetString(k, 10)
			if !flag {
				return fmt.Errorf("format error")
			}
			if new(big.Int).Add(parent.Number, new(big.Int).SetInt64(1)).Cmp(number) >= 0 {
				rwd, success := new(big.Int).SetString(stages[k], 10)
				if !success {
					return fmt.Errorf("format error")
				}

				totalReward := rwd
				//fmt.Println(rwd)
				fmt.Println(totalReward)
				//计算当前周期的单个块的激励
				perBlockReword := totalReward
				fmt.Println(perBlockReword)
				//计算一个周期产生多少块  blockInterval.epochInterval
				cycleBlock := new(big.Int).Div(big.NewInt(epochInterval), big.NewInt(blockInterval))
				fmt.Println(cycleBlock)
				//计算一个周期内总要发送的块激励
				cycleBlockReword := new(big.Int).Mul(perBlockReword, cycleBlock)
				fmt.Println(cycleBlockReword)
				c := len(votes)

				//循环次数
				perCand := cycleBlock.Int64() / int64(c)
				//循环次数后的余数
				reaminderPerCand := cycleBlock.Int64() % int64(c)

				var countNum int64 = 0

				//if !err {
				//	return fmt.Errorf("format error")
				//}
				var countVeryMinNumCount = getDecAdd(int64(c), cycleBlock.Int64())

				fmt.Println("countVeryMinNumCount:", countVeryMinNumCount)

				for candidate := range votes {
					//countVeryMinNum := int64(c) - int64(countNum)
					//fmt.Println("countVeryMinNum:", countVeryMinNum)
					mulPer := (int64(c) - int64(countNum)) * int64(perCand)
					//fmt.Println("mulPer:", mulPer)
					geShu := perCand
					if reaminderPerCand > countNum {
						//mulPer = mulPer + 1
						geShu = geShu + 1
					}

					//fmt.Println("超级节点：", countVeryMinNum)
					//fmt.Println("====：", cycleBlockReword)

					//最终每个节点对应的激励
					nodeBlock := new(big.Int).Mul(new(big.Int).Div(new(big.Int).Div(cycleBlockReword, big.NewInt(countVeryMinNumCount)), big.NewInt(geShu)), big.NewInt(mulPer))
					//fmt.Println("nodeBlock:", nodeBlock)
					//score := votes[candidate]
					ret[candidate] = nodeBlock
					//fmt.Println("totalReward:", totalReward)
					//fmt.Println("", score)
					//fmt.Println("candidate:", candidate)
					countNum = countNum + 1

				}
				//fmt.Println("countVeryMinNum:", countVeryMinNum)
				break
			}
		}
		currentRewardMap = ret
		str, err := json.Marshal(ret)
		if err != nil {
			fmt.Println(err)
		}
		fmt.Println("map to json", string(str))

		epochTrie, _ := types.NewEpochTrie(common.Hash{}, ec.DposContext.DB())
		ec.DposContext.SetEpoch(epochTrie)
		ec.DposContext.SetValidators(sortedValidators)
		log.Info("Come to new epoch", "prevEpoch", i, "nextEpoch", i+1)
	}
	return nil
}

type sortableAddress struct {
	address common.Address
	weight  *big.Int
}
type sortableAddresses []*sortableAddress

func (p sortableAddresses) Swap(i, j int) { p[i], p[j] = p[j], p[i] }
func (p sortableAddresses) Len() int      { return len(p) }
func (p sortableAddresses) Less(i, j int) bool {
	if p[i].weight.Cmp(p[j].weight) < 0 {
		return false
	} else if p[i].weight.Cmp(p[j].weight) > 0 {
		return true
	} else {
		return p[i].address.String() < p[j].address.String()
	}
}
