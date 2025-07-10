package kafka

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"github.com/lib/pq"
	"github.com/netcracker/qubership-maas/dao"
	"github.com/netcracker/qubership-maas/model"
	"github.com/netcracker/qubership-maas/msg"
	"github.com/netcracker/qubership-maas/service/bg2/domain"
	"github.com/netcracker/qubership-maas/utils"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var ErrTopicAlreadyExists = errors.New("dao: kafka topic with such classifier or topic name and instance already exists")
var ErrTopicTemplateAlreadyExists = errors.New("dao: kafka topic template with such name already exists")
var ErrTopicTemplateIsUsedByTopic = errors.New("dao: kafka topic template is used by one or more topics. Update topic without template field to unlink them")

type KafkaDaoImpl struct {
	base           dao.BaseDao
	domainSupplier func(context.Context, string) (*domain.BGNamespaces, error)
}

type TopicDefinitionSearch struct {
	Classifier *model.Classifier
	Namespace  string
	Kind       string
}

func NewKafkaServiceDao(base dao.BaseDao, domainSupplier func(context.Context, string) (*domain.BGNamespaces, error)) *KafkaDaoImpl {
	return &KafkaDaoImpl{base: base, domainSupplier: domainSupplier}
}

func (d *KafkaDaoImpl) WithLock(ctx context.Context, lockId string, f func(ctx context.Context) error) error {
	return d.base.WithLock(ctx, lockId, f)
}

// InsertTopicRegistration insert new registration into database and run `ext' code inside one transaction
func (d *KafkaDaoImpl) InsertTopicRegistration(ctx context.Context, reg *model.TopicRegistration, ext func(reg *model.TopicRegistration) (*model.TopicRegistrationRespDto, error)) (*model.TopicRegistrationRespDto, error) {
	log.InfoC(ctx, "Inserting kafka topic registration in db: %+v", reg)
	var result *model.TopicRegistrationRespDto
	if err := d.base.WithTx(ctx, func(ctx context.Context, cnn *gorm.DB) error {
		var err error
		// create topic registration in old structure for backward compatibility
		if err = cnn.Create(reg).Error; err != nil {
			if d.base.IsUniqIntegrityViolation(err) {
				return utils.LogError(log, ctx, "error during inserting kafka topic registration: %+v, error: %s: %w", reg, err.Error(), msg.Conflict)
			}
			return utils.LogError(log, ctx, "error insert topic: %+v, error: %w", reg, err)
		}

		// insert classifier to new table
		clsEntity := TopicClassifierEntity{
			TopicId:    reg.Id,
			Classifier: *reg.Classifier,
		}
		if err := cnn.Create(&clsEntity).Error; err != nil {
			return utils.LogError(log, ctx, "error insert classifier %+v: %w", clsEntity, err)
		}
		if !reg.Versioned {
			// and add sibling classifier
			if d.domainSupplier != nil {
				domain, err := d.domainSupplier(ctx, reg.Classifier.GetNamespace())
				if err != nil {
					return utils.LogError(log, ctx, "error get domain info: %w", err)
				}
				if domain != nil {
					siblingClassifier := createSiblingClassifier(&clsEntity, domain)
					if err := cnn.Create(&siblingClassifier).Error; err != nil {
						return utils.LogError(log, ctx, "error insert sibling classifier: %w", err)
					}
				}
			}
		}

		if ext != nil {
			result, err = ext(reg)

		}
		return err
	}); err != nil {
		if err == ErrTopicAlreadyExists {
			log.InfoC(ctx, "Kafka topic already exists and was not inserted in db: %v", err.Error())
		} else {
			log.ErrorC(ctx, "Error insert kafka topic in db: %v", err.Error())
		}
		return nil, err
	}
	return result, nil
}

func createSiblingClassifier(entity *TopicClassifierEntity, bgDomain *domain.BGNamespaces) *TopicClassifierEntity {
	siblingClassifier := *entity // create classifier copy
	siblingClassifier.Id = 0     // reset primary key
	if entity.Classifier.GetNamespace() == bgDomain.Origin {
		siblingClassifier.Classifier.Namespace = bgDomain.Peer
	} else if entity.Classifier.GetNamespace() == bgDomain.Peer {
		siblingClassifier.Classifier.Namespace = bgDomain.Origin
	} else {
		log.Panic("classifier namespace %+v doesn't match domain %+v", entity.Classifier, bgDomain)
	}
	return &siblingClassifier
}

func createSiblingDefinitionClassifier(entity *TopicDefinitionClassifierEntity, bgDomain *domain.BGNamespaces) *TopicDefinitionClassifierEntity {
	siblingClassifier := *entity // create classifier copy
	siblingClassifier.Id = 0     // reset primary key
	if entity.Classifier.GetNamespace() == bgDomain.Origin {
		siblingClassifier.Classifier.Namespace = bgDomain.Peer
	} else if entity.Classifier.GetNamespace() == bgDomain.Peer {
		siblingClassifier.Classifier.Namespace = bgDomain.Origin
	} else {
		log.Panic("classifier namespace %+v doesn't match domain %+v", entity.Classifier, bgDomain)
	}
	return &siblingClassifier
}

// UpdateTopicRegistration updates topic registration in database and runs `ext' code inside one transaction.
func (d *KafkaDaoImpl) UpdateTopicRegistration(ctx context.Context, reg *model.TopicRegistration, ext func(reg *model.TopicRegistration) error) error {
	log.InfoC(ctx, "Updating kafka topic registration in db: %v", reg)

	if err := d.base.WithTx(ctx, func(ctx context.Context, cnn *gorm.DB) error {
		if e := cnn.Save(reg).Error; e != nil {
			return e
		}
		if err := ext(reg); err != nil {
			log.ErrorC(ctx, "Error in external function (tx will be rolled back): %v", err.Error())
			return err
		}
		return nil
	}); err != nil {
		log.ErrorC(ctx, "Error updating kafka topic in db: %v", err.Error())
		return err
	}
	return nil
}

// delete registration into database and run `ext' code inside one transaction
func (d *KafkaDaoImpl) DeleteTopicRegistration(ctx context.Context, reg *model.TopicRegistration, ext func(reg *model.TopicRegistration) error) error {
	log.InfoC(ctx, "Deleting kafka topic registration from db: %v", reg)
	if err := d.base.WithTx(ctx, func(ctx context.Context, cnn *gorm.DB) error {
		if e := cnn.Delete(reg).Error; e != nil {
			return e
		}

		if ext != nil {
			return ext(reg)
		}
		return nil
	}); err != nil {
		log.ErrorC(ctx, "Error deleting kafka topic in db: %v", err.Error())
		return err
	}
	return nil
}

func (d *KafkaDaoImpl) FindTopicsBySearchRequest(ctx context.Context, search *model.TopicSearchRequest) ([]*model.TopicRegistration, error) {
	log.InfoC(ctx, "Querying kafka topic registrations by search form %+v", search)
	obj := new([]*model.TopicRegistration)

	if err := d.base.WithTx(ctx, func(ctx context.Context, cnn *gorm.DB) error {
		if err := d.migrateAllBg2Kafka(ctx, cnn); err != nil {
			return err
		}

		query := cnn
		if !search.Classifier.IsEmpty() {
			classifier := search.Classifier.ToJsonString()
			classifiers := cnn.Model(&TopicClassifierEntity{}).Select("classifier").Where("topic_id = kafka_topics.id and classifier = ?::jsonb", classifier)
			query = query.
				//todo add where with md5 index
				//Where("decode(md5(classifier), 'HEX')=decode(md5(?), 'HEX')", classifier).
				Where("(EXISTS(?) OR classifier=?)", classifiers, classifier)
		}
		if search.Instance != "" {
			query = query.Where("instance=?", search.Instance)
		}
		if search.Topic != "" {
			query = query.Where("topic=?", search.Topic)
		}
		if search.Namespace != "" {
			classifiersNamespaces := cnn.Model(&TopicClassifierEntity{}).Select("classifier").Where("topic_id = kafka_topics.id and classifier->>'namespace' = ?", search.Namespace)
			query = query.
				Where("(EXISTS(?) OR namespace=?)", classifiersNamespaces, search.Namespace)
		}
		if search.Template != 0 {
			query = query.Where("template=?", search.Template)
		}

		if search.Versioned != nil {
			query = query.Where("versioned=?", search.Versioned)
		}

		err := query.Find(obj).Error

		if len(*obj) == 0 {
			obj = nil
			return nil
		}
		return err
	}); err != nil {
		log.ErrorC(ctx, "Error query by search form %+v, error: %s", search, err.Error())
		return nil, err
	}
	if obj == nil {
		log.Info("Kafka topics by search form %+v do not exist", search)
		return nil, nil
	}
	log.InfoC(ctx, "Kafka topics by search form %+v were found. Count: %d", search, len(*obj))
	return *obj, nil
}

// templates
func (d *KafkaDaoImpl) FindTopicTemplateByNameAndNamespace(ctx context.Context, name string, namespace string) (*model.TopicTemplate, error) {
	log.InfoC(ctx, "Querying kafka topic template by name: %v", name)
	obj := new(model.TopicTemplate)
	if err := d.base.UsingDb(ctx, func(cnn *gorm.DB) error {
		err := cnn.
			Where("name=?", name).
			Where(cnn.Where("?=ANY(domain_namespaces)", namespace).Or("namespace=?", namespace)).
			First(obj).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			obj = nil
			return nil
		}
		return err
	}); err != nil {
		log.ErrorC(ctx, "Error query topic template by name %v, error: %v", name, err.Error())
		return nil, err
	}
	if obj == nil {
		log.InfoC(ctx, "Kafka topic template with name %v does not exist in db", name)
		return nil, nil
	}
	log.InfoC(ctx, "Kafka topic template with name %v was found: %+v", name, obj)
	return obj, nil
}

func (d *KafkaDaoImpl) InsertTopicTemplate(ctx context.Context, topicTemplate *model.TopicTemplate) error {
	log.InfoC(ctx, "Inserting kafka topic template: %v", topicTemplate)
	if err := d.base.WithTx(ctx, func(ctx context.Context, cnn *gorm.DB) error {

		topicTemplate.DomainNamespaces = []string{topicTemplate.Namespace}
		if d.domainSupplier != nil {
			namespacesPair, err := d.domainSupplier(ctx, topicTemplate.Namespace)
			if err != nil {
				return err
			}
			if namespacesPair != nil {
				topicTemplate.Namespace = namespacesPair.Origin
				topicTemplate.DomainNamespaces = []string{namespacesPair.Origin, namespacesPair.Peer}
			}
		}
		if e := cnn.Create(topicTemplate).Error; e != nil {
			if d.base.IsUniqIntegrityViolation(e) {
				return ErrTopicTemplateAlreadyExists
			}
			return e
		}
		return nil
	}); err != nil {
		log.ErrorC(ctx, "Error insert kafka topic template in db: %v", err.Error())
		return err
	}
	return nil
}

func (d *KafkaDaoImpl) UpdateTopicTemplate(ctx context.Context, topicTemplate *model.TopicTemplate) error {
	log.InfoC(ctx, "Updating kafka topic template: %v", topicTemplate)
	if err := d.base.WithTx(ctx, func(ctx context.Context, cnn *gorm.DB) error {
		if e := cnn.Save(topicTemplate).Error; e != nil {
			if d.base.IsUniqIntegrityViolation(e) {
				return ErrTopicTemplateAlreadyExists
			}
			return e
		}
		return nil
	}); err != nil {
		log.ErrorC(ctx, "Error updating kafka topic template in db: %v", err.Error())
		return err
	}
	return nil
}

func (d *KafkaDaoImpl) MakeTopicsDirtyByTemplate(ctx context.Context, topicTemplate *model.TopicTemplate) error {
	log.InfoC(ctx, "MakeTopicsDirtyByTemplate: %v", topicTemplate)
	var topic model.TopicRegistration
	if err := d.base.WithTx(ctx, func(ctx context.Context, cnn *gorm.DB) error {
		if e := cnn.Raw("UPDATE kafka_topics SET dirty = true WHERE template = ?", topicTemplate.Id).
			Scan(topic).Error; e != nil {
			if d.base.IsUniqIntegrityViolation(e) {
				return ErrTopicTemplateAlreadyExists
			}
			return e
		}
		return nil
	}); err != nil {
		log.ErrorC(ctx, "Error MakeTopicsDirtyByTemplate in db: %v", err.Error())
		return err
	}
	return nil
}

func (d *KafkaDaoImpl) FindTopicsByTemplate(ctx context.Context, topicTemplate *model.TopicTemplate) ([]model.TopicRegistration, error) {
	log.InfoC(ctx, "getting kafka topics by template: %v", topicTemplate)
	obj := new([]model.TopicRegistration)
	if err := d.base.UsingDb(ctx, func(cnn *gorm.DB) error {
		err := cnn.
			Where("template=?", topicTemplate.Id).
			Find(obj).Error
		return err
	}); err != nil {
		log.ErrorC(ctx, "Error query topics by template %v, error: %v", topicTemplate, err.Error())
		return nil, err
	}

	if len(*obj) == 0 {
		log.InfoC(ctx, "Kafka topics with template %v do not exist", topicTemplate)
		return nil, nil
	}

	log.InfoC(ctx, "Kafka topics with template %v were found: %+v", topicTemplate, obj)
	return *obj, nil
}

func (d *KafkaDaoImpl) FindAllTopicTemplatesByNamespace(ctx context.Context, namespace string) ([]model.TopicTemplate, error) {
	log.InfoC(ctx, "getting all kafka topic templates for namespace: `%v'", namespace)
	obj := new([]model.TopicTemplate)
	err := d.base.UsingDb(ctx, func(cnn *gorm.DB) error {
		return cnn.Where("namespace=?", namespace).Find(obj).Error
	})
	if err != nil {
		return nil, utils.LogError(log, ctx, "error query topic templates for namespace `%v': %w", namespace, err)
	}

	log.InfoC(ctx, "topic templates for namespace `%v': %+v", namespace, obj)
	return *obj, nil
}

func (d *KafkaDaoImpl) DeleteTopicTemplate(ctx context.Context, template *model.TopicTemplate) error {
	log.InfoC(ctx, "Deleting kafka topic template: %v", template)
	if err := d.base.WithTx(ctx, func(ctx context.Context, cnn *gorm.DB) error {
		if e := cnn.Delete(template).Error; e != nil {
			return e
		}
		return nil
	}); err != nil {
		if d.base.IsForeignKeyIntegrityViolation(err) {
			log.WarnC(ctx, "topic template is in use: %v", err.Error())
			return ErrTopicTemplateIsUsedByTopic
		}

		log.ErrorC(ctx, "Error deleting kafka topic template in db: %v", err.Error())
		return err
	}
	return nil
}

func (d *KafkaDaoImpl) DeleteTopicTemplatesByNamespace(ctx context.Context, namespace string) error {
	log.DebugC(ctx, "Deleting kafka topic template by namespace: %v", namespace)
	var template model.TopicTemplate
	if err := d.base.WithTx(ctx, func(ctx context.Context, cnn *gorm.DB) error {
		if e := cnn.
			Where("namespace=?", namespace).
			Delete(&template).Error; e != nil {
			return e
		}
		return nil
	}); err != nil {
		if d.base.IsForeignKeyIntegrityViolation(err) {
			log.WarnC(ctx, "topic template is in use during DeleteTopicTemplatesByNamespace: %v", err.Error())
			return ErrTopicTemplateIsUsedByTopic
		}

		log.ErrorC(ctx, "Error deleting kafka topic template in db: %v", err.Error())
		return err
	}
	return nil
}

func (d *KafkaDaoImpl) UpdateTopicDefinition(ctx context.Context, reg *model.TopicDefinition) error {
	log.InfoC(ctx, "Updating kafka TopicDefinition: %+v", reg)
	if err := d.base.WithTx(ctx, func(ctx context.Context, cnn *gorm.DB) error {
		if e := cnn.Where("classifier=?", reg.Classifier).Save(reg).Error; e != nil {
			return e
		}
		return nil
	}); err != nil {
		log.ErrorC(ctx, "Error update TopicDefinition in db: %v", err.Error())
		return err
	}
	return nil
}

func (d *KafkaDaoImpl) InsertTopicDefinition(ctx context.Context, reg *model.TopicDefinition) error {
	log.InfoC(ctx, "Inserting TopicDefinition: %v", reg)
	if err := d.base.WithTx(ctx, func(ctx context.Context, cnn *gorm.DB) error {
		if err := d.migrateAllBg2KafkaTopicDefinitions(ctx, cnn); err != nil {
			return err
		}
		if e := cnn.Create(reg).Error; e != nil {
			if d.base.IsUniqIntegrityViolation(e) {
				return fmt.Errorf("error create topic: %+v, error: %w", reg, ErrTopicAlreadyExists)
			}
			return e
		}

		// insert classifier to new table
		clsEntity := TopicDefinitionClassifierEntity{
			TopicDefinitionId: reg.Id,
			Classifier:        reg.Classifier,
		}
		if err := cnn.Create(&clsEntity).Error; err != nil {
			return utils.LogError(log, ctx, "error insert classifier: %w", err)
		}

		// and add sibling classifier
		if !reg.Versioned && d.domainSupplier != nil {
			domain, err := d.domainSupplier(ctx, reg.Classifier.GetNamespace())
			if err != nil {
				return utils.LogError(log, ctx, "error get domain info: %w", err)
			}
			if domain != nil {
				siblingClassifier := createSiblingDefinitionClassifier(&clsEntity, domain)
				if err := cnn.Create(&siblingClassifier).Error; err != nil {
					return utils.LogError(log, ctx, "error insert sibling classifier: %w", err)
				}
			}
		}
		return nil
	}); err != nil {
		if errors.Is(err, ErrTopicAlreadyExists) {
			log.InfoC(ctx, "Kafka TopicDefinition was not inserted in db because it is already exists: %v", err.Error())
		} else {
			log.ErrorC(ctx, "Error insert kafka TopicDefinition in db: %v", err.Error())
		}
		return err
	}

	return nil
}

func (d *KafkaDaoImpl) FindTopicDefinitions(ctx context.Context, clause TopicDefinitionSearch) ([]model.TopicDefinition, error) {
	log.InfoC(ctx, "List kafka TopicDefinitions by search clause: %+v", clause)
	obj := new([]model.TopicDefinition)
	if err := d.base.UsingDb(ctx, func(cnn *gorm.DB) error {
		if err := d.migrateAllBg2KafkaTopicDefinitions(ctx, cnn); err != nil {
			return err
		}

		classifierMatch := cnn.Model(&TopicDefinitionClassifierEntity{}).
			Where("topic_definition_id = kafka_topic_definitions.id")
		if clause.Classifier != nil {
			classifierMatch = classifierMatch.Where("classifier = ?", clause.Classifier.ToJsonString())
			cnn = cnn.Where("EXISTS(?)", classifierMatch.Select("1"))
		}
		if clause.Namespace != "" {
			classifierMatch = classifierMatch.Where("classifier->>'namespace' = ?", clause.Namespace)
			cnn = cnn.Where("EXISTS(?) or namespace=?", classifierMatch.Select("1"), clause.Namespace)
		}

		if clause.Kind != "" {
			cnn = cnn.Where("kind=?", clause.Kind)
		}

		return cnn.Model(model.TopicDefinition{}).Find(obj).Error
	}); err != nil {
		return nil, utils.LogError(log, ctx, "error query topic definition by clause %+v: %w", clause, err)
	}

	log.InfoC(ctx, "Found Kafka TopicDefinitions by clause %+v: len=%v", clause, len(*obj))
	return *obj, nil
}

func (d *KafkaDaoImpl) DeleteTopicDefinition(ctx context.Context, classifier *model.Classifier) (*model.TopicDefinition, error) {
	log.InfoC(ctx, "Delete kafka TopicDefinition by classifier: %+v", classifier)

	topicDefinition := new(model.TopicDefinition)
	if err := d.base.WithTx(ctx, func(ctx context.Context, cnn *gorm.DB) error {
		if err := d.migrateAllBg2KafkaTopicDefinitions(ctx, cnn); err != nil {
			return err
		}

		return cnn.Where("EXISTS(?)", cnn.Model(&TopicDefinitionClassifierEntity{}).
			Where("topic_definition_id = kafka_topic_definitions.id AND classifier = ?", classifier.ToJsonString()).Select("1")).
			Clauses(clause.Returning{}).
			Delete(topicDefinition).Error
	}); err != nil {
		return nil, utils.LogError(log, ctx, "error delete topic definition by classifier %+v: %w", classifier, err)
	}

	if topicDefinition == nil {
		return nil, utils.LogError(log, ctx, "topic definition not found by %+v: %w", classifier, msg.NotFound)
	}

	return topicDefinition, nil
}

func (d *KafkaDaoImpl) DeleteTopicDefinitionsByNamespace(ctx context.Context, namespace string) error {
	log.InfoC(ctx, "Deleting kafka TopicDefinition by namespace: %v", namespace)
	if err := d.base.WithTx(ctx, func(ctx context.Context, cnn *gorm.DB) error {
		var topicDef model.TopicDefinition
		if e := cnn.
			Where("namespace=?", namespace).
			Delete(topicDef).Error; e != nil {
			return e
		}
		return nil
	}); err != nil {
		log.ErrorC(ctx, "Error deleting kafka TopicDefinitions by namespace in db: %v", err.Error())
		return err
	}
	return nil
}

func (d *KafkaDaoImpl) Warmup(ctx context.Context, origin string, peer string) error {
	log.InfoC(ctx, "Perform warmup procedure for topic classifiers %s:%s", origin, peer)
	return d.base.WithTx(ctx, func(ctx context.Context, conn *gorm.DB) error {
		if err := d.migrateAllBg2Kafka(ctx, conn); err != nil {
			return err
		}

		sql := `insert into kafka_topic_classifiers (topic_id, classifier)
	select t.id as topic_id, t.classifier::jsonb || @sub2 as classifier 
		from kafka_topics as t left join kafka_topic_classifiers as c on t.id = c.topic_id 
		where 
			c.classifier->>'namespace' = @origin_namespace 
            and (t.versioned is null or t.versioned = false)
			and not exists(select 1 from kafka_topic_classifiers as c2 where c.topic_id = c2.topic_id and c2.classifier->>'namespace' = @peer_namespace)
	union
		select t.id as topic_id, t.classifier::jsonb || @sub1 as classifier 
		from kafka_topics as t left join kafka_topic_classifiers as c on t.id = c.topic_id 
		where 
			c.classifier->>'namespace' = @peer_namespace 
            and (t.versioned is null or t.versioned = false)
			and not exists(select 1 from kafka_topic_classifiers as c2 where c.topic_id = c2.topic_id and c2.classifier->>'namespace' = @origin_namespace)`
		params := map[string]any{
			"origin_namespace": origin,
			"peer_namespace":   peer,
			"sub1":             `{"namespace": "` + origin + `"}`,
			"sub2":             `{"namespace": "` + peer + `"}`,
		}
		if err := conn.Exec(sql, params).Error; err != nil {
			return utils.LogError(log, ctx, "error cloning classifiers: %w", err)
		}

		err := d.migrateBg2KafkaTopicDefinitions(ctx, origin, peer)
		if err != nil {
			return utils.LogError(log, ctx, "error cloning topic definition classifiers: %w", err)
		}
		err = d.topicTemplateAttachNamespace(ctx, origin, peer)
		if err != nil {
			return utils.LogError(log, ctx, "error attaching topic template namespaces %s to %s: %w", origin, peer, err)
		}
		return nil
	})
}

func (d *KafkaDaoImpl) Migrate(ctx context.Context) error {
	return d.base.WithTx(ctx, func(ctx context.Context, conn *gorm.DB) error {
		err := d.migrateAllBg2KafkaTopicDefinitions(ctx, conn)
		if err != nil {
			return err
		}
		return d.migrateAllBg2Kafka(ctx, conn)
	})
}

func (d *KafkaDaoImpl) migrateAllBg2Kafka(ctx context.Context, conn *gorm.DB) error {
	// this part is mandatory migration procedure suitable with LTS version rollback
	sql := `insert into kafka_topic_classifiers(topic_id, classifier)
					select t.id as topic_id, t.classifier::jsonb as classifier
						from kafka_topics t left join kafka_topic_classifiers c on t.id = c.topic_id 
						where c.topic_id is null and (t.versioned is null or t.versioned = false)
`
	if err := conn.Exec(sql).Error; err != nil {
		return utils.LogError(log, ctx, "error MigrateKafka topic classifier: %w", err)
	}
	return nil
}

func (d *KafkaDaoImpl) migrateAllBg2KafkaTopicDefinitions(ctx context.Context, conn *gorm.DB) error {
	// this part is mandatory migration procedure suitable with LTS version rollback
	sql := `insert into kafka_topic_definitions_classifiers(topic_definition_id, classifier)
					select t.id as topic_definition_id, t.classifier::jsonb as classifier
						from kafka_topic_definitions t left join kafka_topic_definitions_classifiers c on t.id = c.topic_definition_id 
						where c.topic_definition_id is null and (t.versioned is null or t.versioned = false)
`
	if err := conn.Exec(sql).Error; err != nil {
		return utils.LogError(log, ctx, "error MigrateKafka topic definition classifier: %w", err)
	}
	return nil
}

func (d *KafkaDaoImpl) FindTopicTemplateById(ctx context.Context, id int64) (*model.TopicTemplate, error) {
	log.InfoC(ctx, "getting kafka TopicTemplate by id: %d", id)
	obj := new(model.TopicTemplate)
	if err := d.base.UsingDb(ctx, func(cnn *gorm.DB) error {
		return cnn.First(obj, id).Error
	}); err != nil {
		log.ErrorC(ctx, "Error query TopicTemplate by id %d, error: %w", id, err)
		return nil, err
	}

	return obj, nil
}

func (d *KafkaDaoImpl) migrateBg2KafkaTopicDefinitions(ctx context.Context, origin, peer string) error {
	return d.base.WithTx(ctx, func(ctx context.Context, conn *gorm.DB) error {
		sql := `insert into kafka_topic_definitions_classifiers(topic_definition_id, classifier)
					select t.id as topic_definition_id, t.classifier::jsonb as classifier
						from kafka_topic_definitions t left join kafka_topic_definitions_classifiers c on t.id = c.topic_definition_id
						where c.topic_definition_id is null and (t.versioned is null or t.versioned = false)
`
		if err := conn.Exec(sql).Error; err != nil {
			return utils.LogError(log, ctx, "error MigrateKafka topic definition classifier: %w", err)
		}

		sql = `insert into kafka_topic_definitions_classifiers (topic_definition_id, classifier)
	select t.id as topic_definition_id, t.classifier::jsonb || @sub2 as classifier 
		from kafka_topic_definitions as t left join kafka_topic_definitions_classifiers as c on t.id = c.topic_definition_id 
		where 
			c.classifier->>'namespace' = @origin_namespace 
            and (t.versioned is null or t.versioned = false)
			and not exists(select 1 from kafka_topic_definitions_classifiers as c2 where c.topic_definition_id = c2.topic_definition_id and c2.classifier->>'namespace' = @peer_namespace)
	union
		select t.id as topic_definition_id, t.classifier::jsonb || @sub1 as classifier 
		from kafka_topic_definitions as t left join kafka_topic_definitions_classifiers as c on t.id = c.topic_definition_id 
		where 
			c.classifier->>'namespace' = @peer_namespace 
            and (t.versioned is null or t.versioned = false)
			and not exists(select 1 from kafka_topic_definitions_classifiers as c2 where c.topic_definition_id = c2.topic_definition_id and c2.classifier->>'namespace' = @origin_namespace)`
		params := map[string]any{
			"origin_namespace": origin,
			"peer_namespace":   peer,
			"sub1":             `{"namespace": "` + origin + `"}`,
			"sub2":             `{"namespace": "` + peer + `"}`,
		}
		if err := conn.Exec(sql, params).Error; err != nil {
			return utils.LogError(log, ctx, "error cloning classifiers: %w", err)
		}
		return nil
	})
}

func (d *KafkaDaoImpl) topicTemplateAttachNamespace(ctx context.Context, origin, peer string) error {
	log.InfoC(ctx, "TopicTemplateAttachNamespace for namespace: %s to %s", origin, peer)

	err := d.base.WithTx(ctx, func(ctx context.Context, cnn *gorm.DB) error {
		return cnn.Exec("UPDATE kafka_topic_templates SET domain_namespaces = ? WHERE namespace = ? AND array_length(domain_namespaces, 1) < 2", pq.StringArray{origin, peer}, origin).Error
	})

	if err != nil {
		log.ErrorC(ctx, "Error TopicTemplateAttachNamespace: %v", err.Error())
		return err
	}

	return nil
}

func (d *KafkaDaoImpl) topicTemplateDetachNamespace(ctx context.Context, namespace string) error {
	log.InfoC(ctx, "TopicTemplateDetachNamespace from namespace: %s", namespace)

	err := d.base.WithTx(ctx, func(ctx context.Context, cnn *gorm.DB) error {
		result := cnn.Exec("UPDATE kafka_topic_templates SET "+
			"namespace = (array_remove(domain_namespaces, @ns))[1], "+
			"domain_namespaces = array_remove(domain_namespaces, @ns) WHERE array_length(domain_namespaces, 1) >= 2", sql.Named("ns", namespace),
		)
		if result.RowsAffected == 0 {
			return utils.LogError(log, ctx, "there is no kafka topic templates attached to '%s' namespace: %w", namespace, msg.BadRequest)
		}
		return result.Error
	})

	if err != nil {
		return utils.LogError(log, ctx, "error TopicTemplateDetachNamespace: %w", err)
	}

	return nil
}
