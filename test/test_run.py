import requests
import hashlib
import yaml
from pprint import pprint


def calculate_md5(file_path):
    try:
        with open(file_path, 'rb') as file:
            data = file.read()
            md5_hash = hashlib.md5()
            md5_hash.update(data)
            return md5_hash.hexdigest()
    except IOError as e:
        print(f"文件读取失败：{str(e)}")


def upload_image(api_url, image_path):
    try:
        # 读取图片文件
        with open(image_path, 'rb') as file:
            image_data = file.read()

        # 计算图片的MD5校验码
        image_md5 = calculate_md5(image_path)

        # 构建请求参数
        params = {
            'md5': image_md5  # 添加图片的MD5校验码作为请求参数
        }
        # 构建请求数据
        files = {
            'image': (image_path, image_data),  # 构建表单字段名为'image'的文件数据
        }

        # 发送POST请求
        response = requests.post(api_url, params=params, files=files)

        # 检查响应状态码
        if response.status_code == 200:
            error_code = response.json()['error_code']
            print(
                f"图片 {image_path} 上传成功！返回错误码: {error_code}\t")
            if error_code == 0 or error_code == '0':
                pprint(response.json())
            else:
                print(
                    f"错误信息: { response.json()['error']},请你稍后再试"
                )

        else:
            print(f"图片 {image_path} 上传失败，HTTP状态码：{response.status_code}")
    except IOError as e:
        print(f"图片 {image_path} 读取失败：{str(e)}")


# 读取配置文件
with open("../web-app/etc/imagepredict.yaml", "r") as config_file:
    config = yaml.safe_load(config_file)

# 构建API URL
api_url = f"http://{config['Host']}:{config['Port']}/v1/predict"

# 文件列表
file_list = ["badImage.jpg", "bigImage.jpeg", "nomalImage.jpeg"]

# 依次上传文件
for file in file_list:
    image_path = file  # 替换为实际的图片路径
    upload_image(api_url, image_path)
