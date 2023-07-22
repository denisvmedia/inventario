import React from 'react';

function Errors({ errors }:any) {
  // example of expected errors variable:
  // [
  //   {
  //     error: {
  //       error: {
  //         data: {
  //           name: 'cannot be blank',
  //           address: 'cannot be blank'
  //         }
  //       },
  //       type: 'validation.Errors'
  //     }
  //     status: 'Unprocessable Entity'
  //   }
  // ]

  // eslint-disable-next-line no-console
  console.log(errors);

  if (!Array.isArray(errors) || errors.length === 0) {
    return null;
  }

  let errorMessages: any;

  errors.forEach((e: any) => {
    if (e.error?.error?.data) {
      errorMessages = Object.keys(e.error.error.data)
        .map((key: any) => (
          <li key={key}>{`${key}: ${e.error.error.data[key]}`}</li>
        ));
    } else {
      errorMessages = [e.status];
    }
  });

  return (
    <div>
      <span>Errors:</span>
      <div><ul>{errorMessages}</ul></div>
    </div>
  );
}

export default Errors;
