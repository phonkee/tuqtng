//  Copyright (c) 2013 Couchbase, Inc.
//  Licensed under the Apache License, Version 2.0 (the "License"); you may not use this file
//  except in compliance with the License. You may obtain a copy of the License at
//    http://www.apache.org/licenses/LICENSE-2.0
//  Unless required by applicable law or agreed to in writing, software distributed under the
//  License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND,
//  either express or implied. See the License for the specific language governing permissions
//  and limitations under the License.

package static

import (
	"log"

	// interfaces
	"github.com/couchbaselabs/tuqtng/catalog"
	"github.com/couchbaselabs/tuqtng/executor"
	"github.com/couchbaselabs/tuqtng/network"
	"github.com/couchbaselabs/tuqtng/optimizer"
	"github.com/couchbaselabs/tuqtng/parser"
	"github.com/couchbaselabs/tuqtng/planner"

	// implementations
	explainerExecutor "github.com/couchbaselabs/tuqtng/executor/explainer"
	simpleExecutor "github.com/couchbaselabs/tuqtng/executor/simple"
	simpleOptimizer "github.com/couchbaselabs/tuqtng/optimizer/simple"
	yaccParser "github.com/couchbaselabs/tuqtng/parser/goyacc"
	simplePlanner "github.com/couchbaselabs/tuqtng/planner/simple"
)

type StaticPipeline struct {
	pool      catalog.Pool
	parser    parser.Parser
	planner   planner.Planner
	optimizer optimizer.Optimizer
	executor  executor.Executor
	explainer executor.Executor
}

func NewStaticPipeline(pool catalog.Pool) *StaticPipeline {
	return &StaticPipeline{
		pool:      pool,
		parser:    yaccParser.NewUnqlParser(),
		planner:   simplePlanner.NewSimplePlanner(pool),
		optimizer: simpleOptimizer.NewSimpleOptimizer(),
		executor:  simpleExecutor.NewSimpleExecutor(pool),
		explainer: explainerExecutor.NewExplainerExecutor(),
	}
}

// this construct allows to support alternate types of queries
// that enter the pipeline at different places
// ie, an UNQLASTQueryRequest could bypass the parser
// or an UNQLPlanRequest could bypass parser, planner, and optimizer
func (this *StaticPipeline) DispatchQuery(query network.Query) {
	request := query.Request
	response := query.Response

	switch request := request.(type) {
	case network.UNQLStringQueryRequest:
		ast, err := this.parser.Parse(request.QueryString)
		if err != nil {
			response.SendError(err)
			return
		}

		// perform semantic verification
		err = ast.VerifySemantics()
		if err != nil {
			response.SendError(err)
			return
		}

		planChannel := this.planner.Plan(ast)

		optimalPlan, err := this.optimizer.Optimize(planChannel)
		if err != nil {
			response.SendError(err)
			return
		}

		if ast.IsExplainOnly() {

			log.Printf("trying to explain the plan")
			err = this.explainer.Execute(optimalPlan, query)
			if err != nil {
				response.SendError(err)
				return
			}

		} else {

			err = this.executor.Execute(optimalPlan, query)
			if err != nil {
				response.SendError(err)
				return
			}
		}

	default:
		log.Printf("Unsupported Request Type %T", request)
	}
}
