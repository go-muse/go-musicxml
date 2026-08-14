package xsdgen

import "fmt"

type complexFieldIdentity struct {
	element expandedName
	choice  *complexChoice
}

// validateFlattenableParticle rejects repeated composite particles whose
// members would otherwise become independent slices. Independent slices lose
// both tuple boundaries and the relative order of the original XML children.
func (s *complexStructure) validateFlattenableParticle(
	particle *ParticlePlan,
) error {
	if particle == nil {
		return nil
	}
	if _, ordered := s.choices[particle]; ordered {
		return nil
	}

	if particle.Kind != ParticleElement &&
		particle.Occurrence.Repeated() {
		identities, err := s.particleFieldIdentities(particle)
		if err != nil {
			return err
		}
		if len(identities) > 1 {
			return fmt.Errorf(
				"%w in %s: repeated %s has %d independent fields; configure the type as ordered content",
				ErrAmbiguousComplexContent,
				s.owner,
				particle.Kind,
				len(identities),
			)
		}
	}

	for childIndex := range particle.Children {
		if err := s.validateFlattenableParticle(
			&particle.Children[childIndex],
		); err != nil {
			return err
		}
	}

	return nil
}

func (s *complexStructure) particleFieldIdentities(
	particle *ParticlePlan,
) (map[complexFieldIdentity]struct{}, error) {
	result := make(map[complexFieldIdentity]struct{})
	if particle == nil {
		return result, nil
	}
	if choice, ordered := s.choices[particle]; ordered {
		result[complexFieldIdentity{choice: choice}] = struct{}{}
		return result, nil
	}

	switch particle.Kind {
	case ParticleElement:
		if particle.Element == nil {
			return nil, ErrInvalidElement
		}
		key := expandedName{
			namespace: particle.Element.Name.Namespace,
			local:     particle.Element.Name.Local,
		}
		if choice, found := s.choiceByElement[key]; found {
			result[complexFieldIdentity{choice: choice}] = struct{}{}
		} else {
			result[complexFieldIdentity{element: key}] = struct{}{}
		}

	case ParticleAll, ParticleChoice, ParticleSequence:
		for childIndex := range particle.Children {
			child, err := s.particleFieldIdentities(
				&particle.Children[childIndex],
			)
			if err != nil {
				return nil, err
			}
			for identity := range child {
				result[identity] = struct{}{}
			}
		}

	case ParticleGroup:
		return nil, fmt.Errorf(
			"%w: unexpanded group in %s",
			ErrUnsupportedComplexGeneration,
			s.owner,
		)

	case ParticleAny:
		return nil, fmt.Errorf(
			"%w: xs:any in %s",
			ErrUnsupportedComplexGeneration,
			s.owner,
		)

	default:
		return nil, fmt.Errorf(
			"%w: unknown particle kind %q",
			ErrInvalidParticle,
			particle.Kind,
		)
	}

	return result, nil
}

// orderElementsByParticle derives the XML field order from xs:sequence
// constraints. A stable topological sort keeps the generator deterministic
// while respecting every ordering relationship expressed by the schema.
func (s *complexStructure) orderElementsByParticle(
	particle *ParticlePlan,
) error {
	if len(s.elements) < 2 || particle == nil {
		return nil
	}

	edges := make([]map[int]struct{}, len(s.elements))
	for fieldIndex := range edges {
		edges[fieldIndex] = make(map[int]struct{})
	}
	if err := s.collectParticleOrder(particle, edges); err != nil {
		return err
	}

	indegree := make([]int, len(s.elements))
	for from := range edges {
		for to := range edges[from] {
			if from == to {
				return fmt.Errorf(
					"%w in %s: element occurs at multiple ordered positions; configure the type as ordered content",
					ErrAmbiguousComplexContent,
					s.owner,
				)
			}
			indegree[to]++
		}
	}

	order := make([]int, 0, len(s.elements))
	used := make([]bool, len(s.elements))
	for len(order) != len(s.elements) {
		next := -1
		for fieldIndex := range s.elements {
			if !used[fieldIndex] && indegree[fieldIndex] == 0 {
				next = fieldIndex
				break
			}
		}
		if next < 0 {
			return fmt.Errorf(
				"%w in %s: incompatible element ordering constraints; configure the type as ordered content",
				ErrAmbiguousComplexContent,
				s.owner,
			)
		}

		used[next] = true
		order = append(order, next)
		for successor := range edges[next] {
			indegree[successor]--
		}
	}

	ordered := make([]complexField, len(s.elements))
	for newIndex, oldIndex := range order {
		ordered[newIndex] = s.elements[oldIndex]
	}
	s.elements = ordered
	s.rebuildElementIndexes()

	return nil
}

func (s *complexStructure) collectParticleOrder(
	particle *ParticlePlan,
	edges []map[int]struct{},
) error {
	if particle == nil {
		return nil
	}
	if _, ordered := s.choices[particle]; ordered {
		return nil
	}

	switch particle.Kind {
	case ParticleElement:
		return nil

	case ParticleSequence:
		for childIndex := range particle.Children {
			if err := s.collectParticleOrder(
				&particle.Children[childIndex],
				edges,
			); err != nil {
				return err
			}
		}
		for earlier := range particle.Children {
			earlierFields, err := s.particleFieldIndexes(
				&particle.Children[earlier],
			)
			if err != nil {
				return err
			}
			for later := earlier + 1; later < len(particle.Children); later++ {
				laterFields, err := s.particleFieldIndexes(
					&particle.Children[later],
				)
				if err != nil {
					return err
				}
				for from := range earlierFields {
					for to := range laterFields {
						edges[from][to] = struct{}{}
					}
				}
			}
		}

		return nil

	case ParticleAll, ParticleChoice:
		for childIndex := range particle.Children {
			if err := s.collectParticleOrder(
				&particle.Children[childIndex],
				edges,
			); err != nil {
				return err
			}
		}

		return nil

	case ParticleGroup:
		return fmt.Errorf(
			"%w: unexpanded group in %s",
			ErrUnsupportedComplexGeneration,
			s.owner,
		)

	case ParticleAny:
		return fmt.Errorf(
			"%w: xs:any in %s",
			ErrUnsupportedComplexGeneration,
			s.owner,
		)

	default:
		return fmt.Errorf(
			"%w: unknown particle kind %q",
			ErrInvalidParticle,
			particle.Kind,
		)
	}
}

func (s *complexStructure) particleFieldIndexes(
	particle *ParticlePlan,
) (map[int]struct{}, error) {
	identities, err := s.particleFieldIdentities(particle)
	if err != nil {
		return nil, err
	}

	result := make(map[int]struct{}, len(identities))
	for identity := range identities {
		if identity.choice != nil {
			fieldIndex, found := s.choiceIndexes[identity.choice]
			if !found {
				return nil, ErrInvalidComplexType
			}
			result[fieldIndex] = struct{}{}
			continue
		}

		fieldIndex, found := s.elementIndexes[identity.element]
		if !found {
			return nil, ErrInvalidComplexType
		}
		result[fieldIndex] = struct{}{}
	}

	return result, nil
}

func (s *complexStructure) rebuildElementIndexes() {
	clear(s.elementIndexes)
	clear(s.choiceIndexes)

	for fieldIndex := range s.elements {
		field := &s.elements[fieldIndex]
		switch field.kind {
		case complexFieldElement:
			key := expandedName{
				namespace: field.name.Namespace,
				local:     field.name.Local,
			}
			s.elementIndexes[key] = fieldIndex

		case complexFieldChoice:
			s.choiceIndexes[field.choice] = fieldIndex
		}
	}
}
