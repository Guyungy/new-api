/*
Copyright (C) 2025 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/

import React, { useState } from 'react';
import { Tabs, TabPane } from '@douyinfe/semi-ui';
import { useTranslation } from 'react-i18next';
import UsageLogsTable from '../../components/table/usage-logs';
import { isAdmin } from '../../helpers';

const VIEW_MODE = {
  LOGS: 'logs',
  USER_RANKING: 'user_ranking',
  INTERCEPT_DETAILS: 'intercept_details',
};

const Token = () => {
  const { t } = useTranslation();
  const isAdminUser = isAdmin();
  const [activeTab, setActiveTab] = useState(VIEW_MODE.LOGS);

  return (
    <div className='mt-[60px] px-2'>
      {isAdminUser && (
        <Tabs
          type='line'
          activeKey={activeTab}
          onChange={setActiveTab}
          className='mb-3'
        >
          <TabPane tab={t('日志明细')} itemKey={VIEW_MODE.LOGS} />
          <TabPane tab={t('用户用量排行')} itemKey={VIEW_MODE.USER_RANKING} />
          <TabPane
            tab={t('请求明细')}
            itemKey={VIEW_MODE.INTERCEPT_DETAILS}
          />
        </Tabs>
      )}
      <UsageLogsTable
        key={activeTab}
        fixedViewMode={isAdminUser ? activeTab : VIEW_MODE.LOGS}
        showViewModeSelector={false}
      />
    </div>
  );
};

export default Token;
