package schema

import (
	"errors"
	"github.com/go-pg/migrations/v8"
	"github.com/netcracker/qubership-maas/model"
	"github.com/netcracker/qubership-maas/utils"
)

func init() {
	ctx := utils.CreateContextFromString("db-evo-#9")
	log.InfoC(ctx, "Register db evolution #9")

	migrations.MustRegisterTx(func(db migrations.DB) error {
		log.InfoC(ctx, "Checking if there are equal classifiers for different instances or namespaces")
		query := `
				select classifier, instance, namespace from kafka_topics 
				where classifier in 
					(select classifier from kafka_topics group by classifier having count(classifier) > 1)
				order by classifier, namespace
				`
		var topics []model.TopicRegistration
		result, err := db.Query(&topics, query)
		if err != nil {
			return err
		}
		if result.RowsReturned() != 0 {
			log.ErrorC(ctx, "Some topics have more than one instance or namespace for one classifier:")
			for _, topic := range topics {
				log.ErrorC(ctx, "classifier '%v', instance '%v', namespace '%v'", topic.Classifier, topic.Instance, topic.Namespace)
			}
			log.ErrorC(ctx, "Some topics have more than one instance or namespace for one classifier, classifier should be unique. You can delete every single topic registrations manually, or you can delete all topics for particular instance OR namespace (or both simultaneously) in one request using filters via delete topic API (it is described in rest_api.md)")
			return errors.New("some topics have more than one instance for one classifier, see logs for more info")
		}

		log.InfoC(ctx, "Changing 'kafka_topics' primary key on  (classifier, instance, namespace) to  primary key on  (classifier)...")
		log.InfoC(ctx, "Drop pkey constraint...")
		if _, err := db.Exec("ALTER TABLE kafka_topics DROP CONSTRAINT kafka_topics_pkey"); err != nil {
			return err
		}

		log.InfoC(ctx, "Creating new primary key...")
		if _, err := db.Exec("ALTER TABLE kafka_topics ADD PRIMARY KEY (classifier)"); err != nil {
			return err
		}

		log.InfoC(ctx, "Evolution #9 successfully finished")
		return nil
	})
}
