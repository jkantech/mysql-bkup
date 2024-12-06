// Package internal /
package internal

/*
MIT License

Copyright (c) 2023 Jonas Kaninda

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
*/
import (
	"github.com/jkaninda/encryptor"
	"github.com/jkaninda/go-storage/pkg/local"
	"github.com/jkaninda/mysql-bkup/pkg/logger"
	"github.com/jkaninda/mysql-bkup/utils"
	"github.com/spf13/cobra"
	"os"
	"os/exec"
	"path/filepath"
)

func StartRestore(cmd *cobra.Command) {
	intro()
	dbConf = initDbConfig(cmd)
	restoreConf := initRestoreConfig(cmd)

	switch restoreConf.storage {
	case "local":
		localRestore(dbConf, restoreConf)
	case "s3", "S3":
		s3Restore(dbConf, restoreConf)
	case "ssh", "SSH", "remote":
		remoteRestore(dbConf, restoreConf)
	case "ftp", "FTP":
		ftpRestore(dbConf, restoreConf)
	case "azure":
		azureRestore(dbConf, restoreConf)
	default:
		localRestore(dbConf, restoreConf)
	}
}
func localRestore(dbConf *dbConfig, restoreConf *RestoreConfig) {
	logger.Info("Restore database from local")
	localStorage := local.NewStorage(local.Config{
		RemotePath: storagePath,
		LocalPath:  tmpPath,
	})
	err := localStorage.CopyFrom(restoreConf.file)
	if err != nil {
		logger.Fatal("Error copying backup file: %s", err)
	}
	RestoreDatabase(dbConf, restoreConf)

}

// RestoreDatabase restore database
func RestoreDatabase(db *dbConfig, conf *RestoreConfig) {
	if conf.file == "" {
		logger.Fatal("Error, file required")
	}
	extension := filepath.Ext(filepath.Join(tmpPath, conf.file))
	rFile, err := os.ReadFile(filepath.Join(tmpPath, conf.file))
	outputFile := RemoveLastExtension(filepath.Join(tmpPath, conf.file))
	if err != nil {
		logger.Fatal("Error reading backup file: %s ", err)
	}

	if extension == ".gpg" {

		if conf.usingKey {
			logger.Info("Decrypting backup using private key...")
			logger.Warn("Backup decryption using a private key is not fully supported")
			prKey, err := os.ReadFile(conf.privateKey)
			if err != nil {
				logger.Fatal("Error reading public key: %s ", err)
			}
			err = encryptor.DecryptWithPrivateKey(rFile, outputFile, prKey, conf.passphrase)
			if err != nil {
				logger.Fatal("error during decrypting backup %v", err)
			}
			logger.Info("Decrypting backup using private key...done")
		} else {
			if conf.passphrase == "" {
				logger.Error("Error, passphrase or private key required")
				logger.Fatal("Your file seems to be a GPG file.\nYou need to provide GPG keys. GPG_PASSPHRASE or GPG_PRIVATE_KEY environment variable is required.")
			} else {
				logger.Info("Decrypting backup using passphrase...")
				// decryptWithGPG file
				err := encryptor.Decrypt(rFile, outputFile, conf.passphrase)
				if err != nil {
					logger.Fatal("Error decrypting file %s %v", file, err)
				}
				logger.Info("Decrypting backup using passphrase...done")
				// Update file name
				conf.file = RemoveLastExtension(file)
			}
		}

	}

	if utils.FileExists(filepath.Join(tmpPath, conf.file)) {
		err := os.Setenv("MYSQL_PWD", db.dbPassword)
		if err != nil {
			return
		}
		testDatabaseConnection(db)
		logger.Info("Restoring database...")

		extension := filepath.Ext(filepath.Join(tmpPath, conf.file))
		// Restore from compressed file / .sql.gz
		if extension == ".gz" {
			str := "zcat " + filepath.Join(tmpPath, conf.file) + " | mysql -h " + db.dbHost + " -P " + db.dbPort + " -u " + db.dbUserName + " " + db.dbName
			_, err := exec.Command("sh", "-c", str).Output()
			if err != nil {
				logger.Fatal("Error, in restoring the database  %v", err)
			}
			logger.Info("Restoring database... done")
			logger.Info("Database has been restored")
			// Delete temp
			deleteTemp()

		} else if extension == ".sql" {
			// Restore from sql file
			str := "cat " + filepath.Join(tmpPath, conf.file) + " | mysql -h " + db.dbHost + " -P " + db.dbPort + " -u " + db.dbUserName + " " + db.dbName
			_, err := exec.Command("sh", "-c", str).Output()
			if err != nil {
				logger.Fatal("Error in restoring the database %v", err)
			}
			logger.Info("Restoring database... done")
			logger.Info("Database has been restored")
			// Delete temp
			deleteTemp()
		} else {
			logger.Fatal("Unknown file extension %s", extension)
		}

	} else {
		logger.Fatal("File not found in %s", filepath.Join(tmpPath, conf.file))
	}
}