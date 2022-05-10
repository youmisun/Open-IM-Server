package admin

import (
	apiStruct "Open_IM/pkg/cms_api_struct"
	"Open_IM/pkg/common/config"
	"Open_IM/pkg/common/constant"
	imdb "Open_IM/pkg/common/db/mysql_model/im_mysql_model"
	openIMHttp "Open_IM/pkg/common/http"
	"Open_IM/pkg/common/log"
	"Open_IM/pkg/grpc-etcdv3/getcdv3"
	pbAdmin "Open_IM/pkg/proto/admin_cms"
	"Open_IM/pkg/utils"
	"context"
	"fmt"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"net/http"
	"path"
	"strings"

	"github.com/gin-gonic/gin"
	url2 "net/url"
)

var (
	minioClient *minio.Client
)

func init() {
	operationID := utils.OperationIDGenerator()
	log.NewInfo(operationID, utils.GetSelfFuncName(), "minio config: ", config.Config.Credential.Minio)
	var initUrl string
	if config.Config.Credential.Minio.EndpointInnerEnable {
		initUrl = config.Config.Credential.Minio.EndpointInner
	} else {
		initUrl = config.Config.Credential.Minio.Endpoint
	}
	log.NewInfo(operationID, utils.GetSelfFuncName(), "use initUrl: ", initUrl)
	minioUrl, err := url2.Parse(initUrl)
	if err != nil {
		log.NewError(operationID, utils.GetSelfFuncName(), "parse failed, please check config/config.yaml", err.Error())
		return
	}
	opts := &minio.Options{
		Creds: credentials.NewStaticV4(config.Config.Credential.Minio.AccessKeyID, config.Config.Credential.Minio.SecretAccessKey, ""),
	}
	if minioUrl.Scheme == "http" {
		opts.Secure = false
	} else if minioUrl.Scheme == "https" {
		opts.Secure = true
	}
	log.NewInfo(operationID, utils.GetSelfFuncName(), "Parse ok ", config.Config.Credential.Minio)
	minioClient, err = minio.New(minioUrl.Host, opts)
	log.NewInfo(operationID, utils.GetSelfFuncName(), "new ok ", config.Config.Credential.Minio)
	if err != nil {
		log.NewError(operationID, utils.GetSelfFuncName(), "init minio client failed", err.Error())
		return
	}
}

// register
func AdminLogin(c *gin.Context) {
	var (
		req   apiStruct.AdminLoginRequest
		resp  apiStruct.AdminLoginResponse
		reqPb pbAdmin.AdminLoginReq
	)
	if err := c.BindJSON(&req); err != nil {
		log.NewInfo("0", utils.GetSelfFuncName(), err.Error())
		openIMHttp.RespHttp200(c, constant.ErrArgs, nil)
		return
	}
	reqPb.Secret = req.Secret
	reqPb.AdminID = req.AdminName
	etcdConn := getcdv3.GetConn(config.Config.Etcd.EtcdSchema, strings.Join(config.Config.Etcd.EtcdAddr, ","), config.Config.RpcRegisterName.OpenImAdminCMSName)
	client := pbAdmin.NewAdminCMSClient(etcdConn)
	respPb, err := client.AdminLogin(context.Background(), &reqPb)
	if err != nil {
		log.NewError(reqPb.OperationID, utils.GetSelfFuncName(), "rpc failed", err.Error())
		openIMHttp.RespHttp200(c, err, nil)
		return
	}
	resp.Token = respPb.Token
	openIMHttp.RespHttp200(c, constant.OK, resp)
}

func UploadUpdateApp(c *gin.Context) {
	var (
		req  apiStruct.UploadUpdateAppReq
		resp apiStruct.UploadUpdateAppResp
	)
	if err := c.Bind(&req); err != nil {
		log.NewError(req.OperationID, utils.GetSelfFuncName(), "BindJSON failed ", err.Error())
		c.JSON(http.StatusBadRequest, gin.H{"errCode": 400, "errMsg": err.Error()})
		return
	}
	log.NewInfo(req.OperationID, utils.GetSelfFuncName(), "req: ", req)

	//fileObj, err := req.File.Open()
	//if err != nil {
	//	log.NewError(req.OperationID, utils.GetSelfFuncName(), "Open file error", err.Error())
	//	c.JSON(http.StatusBadRequest, gin.H{"errCode": 400, "errMsg": "Open file error" + err.Error()})
	//	return
	//}
	//yamlObj, err := req.Yaml.Open()
	//if err != nil {
	//	log.NewError(req.OperationID, utils.GetSelfFuncName(), "Open Yaml error", err.Error())
	//	c.JSON(http.StatusBadRequest, gin.H{"errCode": 400, "errMsg": "Open Yaml error" + err.Error()})
	//	return
	//}

	// v2.0.9_app_linux v2.0.9_yaml_linux
	file, err := c.FormFile("file")
	if err != nil {
		log.NewError(req.OperationID, utils.GetSelfFuncName(), "FormFile failed", err.Error())
		c.JSON(http.StatusBadRequest, gin.H{"errCode": 400, "errMsg": "missing file arg: " + err.Error()})
		return
	}
	fileObj, err := file.Open()
	if err != nil {
		log.NewError(req.OperationID, utils.GetSelfFuncName(), "Open file error", err.Error())
		c.JSON(http.StatusBadRequest, gin.H{"errCode": 400, "errMsg": "invalid file path" + err.Error()})
		return
	}

	yaml, err := c.FormFile("yaml")
	if err != nil {
		log.NewError(req.OperationID, utils.GetSelfFuncName(), "FormFile failed", err.Error())
		c.JSON(http.StatusBadRequest, gin.H{"errCode": 400, "errMsg": "missing file arg: " + err.Error()})
		return
	}
	yamlObj, err := yaml.Open()
	if err != nil {
		log.NewError(req.OperationID, utils.GetSelfFuncName(), "Open file error", err.Error())
		c.JSON(http.StatusBadRequest, gin.H{"errCode": 400, "errMsg": "invalid file path" + err.Error()})
		return
	}
	newFileName, newYamlName, err := utils.GetUploadAppNewName(req.Type, req.Version, file.Filename, yaml.Filename)
	if err != nil {
		log.NewError(req.OperationID, utils.GetSelfFuncName(), "GetUploadAppNewName failed", err.Error())
		c.JSON(http.StatusBadRequest, gin.H{"errCode": 400, "errMsg": "invalid file type" + err.Error()})
		return
	}

	fmt.Println(req.OperationID, utils.GetSelfFuncName(), "name: ", config.Config.Credential.Minio.AppBucket, newFileName, fileObj, file.Size)
	fmt.Println(req.OperationID, utils.GetSelfFuncName(), "name: ", config.Config.Credential.Minio.AppBucket, newYamlName, yamlObj, yaml.Size)

	_, err = minioClient.PutObject(context.Background(), config.Config.Credential.Minio.AppBucket, newFileName, fileObj, file.Size, minio.PutObjectOptions{ContentType: path.Ext(newFileName)})
	if err != nil {
		log.NewError(req.OperationID, utils.GetSelfFuncName(), "PutObject file error")
		c.JSON(http.StatusInternalServerError, gin.H{"errCode": 500, "errMsg": "PutObject file error" + err.Error()})
		return
	}
	_, err = minioClient.PutObject(context.Background(), config.Config.Credential.Minio.AppBucket, newYamlName, yamlObj, yaml.Size, minio.PutObjectOptions{ContentType: path.Ext(newYamlName)})
	if err != nil {
		log.NewError(req.OperationID, utils.GetSelfFuncName(), "PutObject yaml error")
		c.JSON(http.StatusInternalServerError, gin.H{"errCode": 500, "errMsg": "PutObject yaml error" + err.Error()})
		return
	}
	if err := imdb.UpdateAppVersion(req.Type, req.Version, req.ForceUpdate, newFileName, newYamlName); err != nil {
		log.NewError(req.OperationID, utils.GetSelfFuncName(), "UpdateAppVersion error", err.Error())
		resp.ErrCode = http.StatusInternalServerError
		resp.ErrMsg = err.Error()
		c.JSON(http.StatusInternalServerError, resp)
		return
	}
	log.NewInfo(req.OperationID, utils.GetSelfFuncName())
	c.JSON(http.StatusOK, resp)
}

func GetDownloadURL(c *gin.Context) {
	var (
		req  apiStruct.GetDownloadURLReq
		resp apiStruct.GetDownloadURLResp
	)
	defer func() {
		log.NewInfo(req.OperationID, utils.GetSelfFuncName(), "resp: ", resp)
	}()
	if err := c.Bind(&req); err != nil {
		log.NewError("0", utils.GetSelfFuncName(), "BindJSON failed ", err.Error())
		c.JSON(http.StatusBadRequest, gin.H{"errCode": 400, "errMsg": err.Error()})
		return
	}
	log.NewInfo(req.OperationID, utils.GetSelfFuncName(), "req: ", req)
	app, err := imdb.GetNewestVersion(req.Type)
	if err != nil {
		log.NewError(req.OperationID, utils.GetSelfFuncName(), "getNewestVersion failed", err.Error())
	}
	if app != nil {
		if app.Version != req.Version && app.Version != "" {
			resp.Data.HasNewVersion = true
			if app.ForceUpdate == true {
				resp.Data.ForceUpdate = true
			}
			resp.Data.YamlURL = config.Config.Credential.Minio.Endpoint + "/" + config.Config.Credential.Minio.AppBucket + "/" + app.YamlName
			resp.Data.FileURL = config.Config.Credential.Minio.Endpoint + "/" + config.Config.Credential.Minio.AppBucket + "/" + app.FileName
			c.JSON(http.StatusOK, resp)
			return
		} else {
			resp.Data.HasNewVersion = false
			c.JSON(http.StatusOK, resp)
			return
		}
	}
	c.JSON(http.StatusBadRequest, gin.H{"errCode": 0, "errMsg": "not found app version"})
}
