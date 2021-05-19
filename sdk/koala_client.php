<?php
/**
 * Koala Rule Engine PHP SDK
 *
 * @package: main
 * @desc: koala engine - PHP SDK code
 *
 * @author: heiyeluren 
 * @github: https://github.com/heiyeluren
 * @blog: https://blog.csdn.net/heiyeshuwu
 *
 */


/**
 * 调用 koala 服务的 sdk 代码
 */

class KoalaClient {

    private $_host = '';

    private $_port = 0;

    private $_server = '';

    /**
     * $conf = array(
     *     'host'  => '127.0.0.1',
     *     'port'  => 8633,
     * );
    */
    public function __construct($conf) {/*{{{*/
        if (!isset($conf['host']) || !isset($conf['port'])) {
            throw new Exception("err config");
        }
        $this->_host = $conf['host'];
        $this->_port = $conf['port'];
        $this->_server = sprintf("%s:%d", $this->_host, $this->_port);
    }/*}}}*/

    public function check($param) {/*{{{*/
        foreach ($param as $key => &$value) {
            $value = (string)$value;
        }
        $url = $this->_server."/rule/browse";
        $koala_res = $this->send($url, $param);
        $result = array(
            'errno'     => $koala_res['Err_no'],
            'errmsg'    => $koala_res['Str_reason'],
            'code'      => $koala_res['Ret_code'],
            'vcode_len' => $koala_res['Vcode_len'],
        );
        return $result;
    }/*}}}*/

    public function check_complete($param) {/*{{{*/
        foreach ($param as $key => &$value) {
            $value = (string)$value;
        }
        $url = $this->_server."/rule/browse_complete";
        $koala_res = $this->send($url, $param);
        if(empty($koala_res)) {
            return false;
        }
        $result = array();
        foreach ($koala_res as $index => $value) {
            $one_res = array(
                'errno'     => $value['Err_no'],
                'errmsg'    => $value['Str_reason'],
                'code'      => $value['Ret_code'],
                'vcode_len' => $value['Vcode_len'],
            );
            $result[$index] = $one_res;
        }
        return $result;
    }/*}}}*/

    public function multiCheck($jobParam) {/*{{{*/
        $url = $this->_server."/multi/browse";
        $jobs = array();
        foreach ($jobParam as $index => $item) {
            foreach ($item as $key => &$value) {
                $value = (string)$value;
            }
            $jobs[$index]['Id'] = (string)$index;
            $jobs[$index]['Arg'] = urlencode(http_build_query($item));
        }
        $jobs = array_merge($jobs);
        $koala_res = $this->send($url, array('argsJson' => json_encode($jobs)));
        $retArray = array();
        foreach ($koala_res as $item) {
            $id = (int)$item['Id'];
            $retArray[$id] = array(
                'errno'     => $item['Result']['Err_no'],
                'errmsg'    => $item['Result']['Str_reason'],
                'code'      => $item['Result']['Ret_code'],
                'vcode_len' => $item['Result']['Vcode_len'],
            );
        }
        return $retArray;
    }/*}}}*/

    public function write($param) {/*{{{*/
        foreach ($param as $key => &$value) {
            $value = (string)$value;
        }
        $url = $this->_server."/rule/update";
        $koala_res = $this->send($url, $param);
        $result = array(
            'errno'     => $koala_res['err_no'],
            'errmsg'    => $koala_res['err_msg'],
        );
        return $result;
    }/*}}}*/

    public function monitorAlive($param=array()) {/*{{{*/
        $url = $this->_server."/monitor/alive";
        $koala_res = $this->send($url, $param);
        return $koala_res;
    }/*}}}*/

    public function send($url, $param) {/*{{{*/
        $ch = curl_init();
        curl_setopt($ch,CURLOPT_URL, $url.'?'.http_build_query($param));
        curl_setopt($ch,CURLOPT_TIMEOUT,3);
        curl_setopt($ch,CURLOPT_RETURNTRANSFER,true);
        curl_setopt($ch,CURLOPT_HTTPHEADER,array("Host:".$this->_host));
        $rs = curl_exec($ch);
        curl_close($ch);

        return json_decode($rs,true);
    }/*}}}*/

}

